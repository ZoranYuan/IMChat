package kafka

import (
	configs "IM_backend/configs"
	eventbus "IM_backend/internal/application/ports/eventbus"
	inboxport "IM_backend/internal/application/ports/inbox"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
)

var (
	ErrHandlerNotFound = errors.New("消息处理器不存在")
	ErrInvalidClient   = errors.New("消息队列客户端无效")
	ErrEmptyTopics     = errors.New("消息主题不能为空")
)

type ConsumerRouter struct {
	handlers        map[string]eventbus.Handler
	inbox           inboxport.InboxRepository
	txManager       txmanager.TxManager
	inboxStaleAfter time.Duration
}

func NewConsumerRouter(handlers map[string]eventbus.Handler, inbox inboxport.InboxRepository, txManager txmanager.TxManager, config configs.KafkaConsumerConfig) *ConsumerRouter {
	registered := make(map[string]eventbus.Handler, len(handlers))
	for eventName, handler := range handlers {
		if eventName == "" || handler == nil {
			continue
		}

		registered[eventName] = handler
	}
	return &ConsumerRouter{
		handlers:        registered,
		inbox:           inbox,
		txManager:       txManager,
		inboxStaleAfter: time.Duration(config.InboxStaleAfterSecs) * time.Second,
	}
}

func (r *ConsumerRouter) handle(ctx context.Context, message eventbus.IncomingEvent) (string, int, error) {
	if r == nil {
		return "", 0, ErrHandlerNotFound
	}

	handler, ok := r.handlers[message.Name]
	if !ok {
		return "", 0, ErrHandlerNotFound
	}

	claimed := true
	lockToken := ""
	retryCount := 1
	if r.inbox != nil && message.EventID != "" {
		if r.txManager == nil {
			return "", 0, errors.New("Inbox 事务管理器未配置")
		}
		now := time.Now()
		var err error
		err = r.txManager.WithinTransaction(ctx, func(tx any) error {
			claimed, lockToken, retryCount, err = r.inbox.WithTx(tx).TryClaim(ctx, message.EventID, string(message.Name), now, now.Add(-r.inboxStaleAfter))
			return err
		})
		if err != nil {
			return "", retryCount, err
		}
		if !claimed {
			return "", retryCount, nil
		}
	}

	err := handler.Handle(ctx, message)
	if err != nil {
		return lockToken, retryCount, err
	}

	if r.inbox != nil && message.EventID != "" {
		if err := r.inbox.MarkCompleted(ctx, message.EventID, lockToken, time.Now()); err != nil {
			return lockToken, retryCount, fmt.Errorf("标记 Inbox 完成状态失败：%w", err)
		}
	}
	return lockToken, retryCount, nil
}

func (r *ConsumerRouter) markDead(ctx context.Context, message eventbus.IncomingEvent, lockToken, lastError string) error {
	if r == nil || r.inbox == nil || message.EventID == "" {
		return nil
	}
	return r.inbox.MarkDead(ctx, message.EventID, lockToken, lastError, time.Now())
}

type saramaAdapter struct {
	router              *ConsumerRouter
	topicRouter         *TopicRouter
	maxInboxRetries     int
	deadLetterPublisher eventbus.Publisher
	deadLetterSuffix    string
}

func cloneBytes(data []byte) []byte {
	if data == nil {
		return nil
	}

	copied := make([]byte, len(data))
	copy(copied, data)

	return copied
}

func eventIDFromHeaders(headers []*sarama.RecordHeader) string {
	for _, header := range headers {
		if header != nil && string(header.Key) == "event_id" {
			return string(header.Value)
		}
	}
	return ""
}

func (saramaAdapter) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (saramaAdapter) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h saramaAdapter) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for {
		select {
		case <-session.Context().Done():
			return session.Context().Err()
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}

			eventName, mapErr := h.topicRouter.EventFor(message.Topic)
			if mapErr != nil {
				return mapErr
			}

			lockToken, retryCount, err := h.router.handle(session.Context(),
				eventbus.IncomingEvent{
					EventID: eventIDFromHeaders(message.Headers),
					Name:    eventName,

					// 复制数据，避免业务层继续持有 Sarama 内部消息切片。
					Key:     cloneBytes(message.Key),
					Payload: cloneBytes(message.Value),
				})

			if err != nil {
				if eventbus.IsNonRetryable(err) || (h.maxInboxRetries > 0 && retryCount >= h.maxInboxRetries) {
					if h.deadLetterPublisher == nil {
						return fmt.Errorf("死信发布器未配置：%w", err)
					}
					if publishErr := h.deadLetterPublisher.Publish(
						session.Context(),
						eventbus.IntegrationEvent{
							EventID:      eventIDFromHeaders(message.Headers),
							Name:         eventName + h.deadLetterSuffix,
							PartitionKey: string(message.Key),
							Payload:      cloneBytes(message.Value),
						},
					); publishErr != nil {
						return fmt.Errorf("发布死信消息失败：%w", publishErr)
					}
					if markErr := h.router.markDead(session.Context(), eventbus.IncomingEvent{
						EventID: eventIDFromHeaders(message.Headers),
						Name:    eventName,
					}, lockToken, err.Error()); markErr != nil {
						return fmt.Errorf("标记 Inbox 死信状态失败：%w", markErr)
					}
					log.Printf(
						"消息已转入死信队列：主题=%s 分区=%d 偏移量=%d 错误=%v",
						message.Topic,
						message.Partition,
						message.Offset,
						err,
					)
					session.MarkMessage(message, "")
					continue
				}
				// 不调用 MarkMessage，当前 offset 不会被当前处理流程提交。
				return fmt.Errorf(
					"处理消息队列消息失败：主题=%s 分区=%d 偏移量=%d：%w",
					message.Topic,
					message.Partition,
					message.Offset,
					err,
				)
			}

			session.MarkMessage(message, "")
		}
	}
}

type ConsumerGroup struct {
	group               sarama.ConsumerGroup
	topicRouter         *TopicRouter
	maxInboxRetries     int
	topics              []string
	handlerRouter       *ConsumerRouter
	retryInterval       time.Duration
	deadLetterPublisher eventbus.Publisher
	deadLetterSuffix    string
}

func normalizeTopics(topics []string) []string {
	result := make([]string, 0, len(topics))
	seen := make(map[string]struct{}, len(topics))

	for _, topic := range topics {
		if topic == "" {
			continue
		}

		if _, exists := seen[topic]; exists {
			continue
		}

		seen[topic] = struct{}{}
		result = append(result, topic)
	}

	return result
}

func NewConsumerGroup(
	client *Client,
	topics []string,
	router *ConsumerRouter,
	deadLetterPublisher eventbus.Publisher,
	topicRouter *TopicRouter,
	config configs.KafkaConsumerConfig,
) (*ConsumerGroup, error) {
	if client == nil || client.Consumer == nil {
		return nil, ErrInvalidClient
	}

	validTopics := normalizeTopics(topics)
	if len(validTopics) == 0 {
		return nil, ErrEmptyTopics
	}

	if router == nil || topicRouter == nil {
		return nil, errors.New("Kafka Consumer 路由器未配置")
	}
	if config.MaxInboxRetries <= 0 || config.InboxStaleAfterSecs <= 0 || config.ConsumeRetryIntervalSecs <= 0 {
		return nil, errors.New("Kafka Consumer 重试配置无效")
	}
	if config.DeadLetterSuffix == "" {
		return nil, errors.New("Kafka DLQ 后缀未配置")
	}

	group := &ConsumerGroup{
		group:               client.Consumer,
		topics:              validTopics,
		handlerRouter:       router,
		topicRouter:         topicRouter,
		maxInboxRetries:     config.MaxInboxRetries,
		retryInterval:       time.Duration(config.ConsumeRetryIntervalSecs) * time.Second,
		deadLetterPublisher: deadLetterPublisher,
		deadLetterSuffix:    config.DeadLetterSuffix,
	}
	return group, nil
}

func (c *ConsumerGroup) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	handler := saramaAdapter{
		router:              c.handlerRouter,
		maxInboxRetries:     c.maxInboxRetries,
		deadLetterPublisher: c.deadLetterPublisher,
		deadLetterSuffix:    c.deadLetterSuffix,
	}
	for ctx.Err() == nil {
		if err := c.group.Consume(ctx, c.topics, handler); err != nil {
			log.Printf(
				"消费消息主题失败：主题=%v，重试间隔=%s，错误=%v",
				c.topics,
				c.retryInterval,
				err,
			)

			select {
			case <-ctx.Done():
				return context.Cause(ctx)
			case <-time.After(c.retryInterval):
			}
		}
	}
	// 返回上下文取消的具体原因
	return context.Cause(ctx)
}

func (c *ConsumerGroup) Close() error {
	if c == nil || c.group == nil {
		return nil
	}

	return c.group.Close()
}
