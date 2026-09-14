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
	handlers            map[string]eventbus.Handler
	inbox               inboxport.InboxRepository
	txManager           txmanager.TxManager
	inboxStaleAfter     time.Duration
	maxInboxRetries     int
	deadLetterPublisher eventbus.Publisher
	deadLetterSuffix    string
}

type inboxClaim struct {
	claimed    bool
	lockToken  string
	retryCount int
}

func NewConsumerRouter(
	handlers map[string]eventbus.Handler,
	inbox inboxport.InboxRepository,
	txManager txmanager.TxManager,
	config configs.KafkaConsumerConfig,
	deadLetterPublisher eventbus.Publisher,
) *ConsumerRouter {
	registered := make(map[string]eventbus.Handler, len(handlers))
	for eventName, handler := range handlers {
		if eventName == "" || handler == nil {
			continue
		}

		registered[eventName] = handler
	}
	return &ConsumerRouter{
		handlers:            registered,
		inbox:               inbox,
		txManager:           txManager,
		inboxStaleAfter:     time.Duration(config.InboxStaleAfterSecs) * time.Second,
		maxInboxRetries:     config.MaxInboxRetries,
		deadLetterPublisher: deadLetterPublisher,
		deadLetterSuffix:    config.DeadLetterSuffix,
	}
}

func (r *ConsumerRouter) claim(ctx context.Context, message eventbus.IncomingEvent) (inboxClaim, error) {
	if r == nil {
		return inboxClaim{}, ErrHandlerNotFound
	}

	if _, ok := r.handlers[message.Name]; !ok {
		return inboxClaim{}, eventbus.NonRetryable(ErrHandlerNotFound)
	}

	claim := inboxClaim{
		claimed:    true,
		retryCount: 1,
	}
	if r.inbox != nil && message.EventID != "" {
		if r.txManager == nil {
			return inboxClaim{}, errors.New("Inbox 事务管理器未配置")
		}
		now := time.Now()
		err := r.txManager.WithinTransaction(ctx, func(tx any) error {
			var err error
			claim.claimed, claim.lockToken, claim.retryCount, err = r.inbox.WithTx(tx).TryClaim(
				ctx,
				message.EventID,
				string(message.Name),
				now,
				now.Add(-r.inboxStaleAfter),
			)
			return err
		})
		if err != nil {
			return claim, err
		}
	}
	return claim, nil
}

func (r *ConsumerRouter) execute(ctx context.Context, message eventbus.IncomingEvent) error {
	if r == nil {
		return ErrHandlerNotFound
	}

	handler, ok := r.handlers[message.Name]
	if !ok {
		return eventbus.NonRetryable(ErrHandlerNotFound)
	}
	return handler.Handle(ctx, message)
}

func (r *ConsumerRouter) publishDeadLetter(ctx context.Context, message eventbus.IncomingEvent) error {
	if r == nil || r.deadLetterPublisher == nil {
		return errors.New("死信发布器未配置")
	}

	if err := r.deadLetterPublisher.Publish(
		ctx,
		eventbus.IntegrationEvent{
			EventID:      message.EventID,
			Name:         message.Name + r.deadLetterSuffix,
			PartitionKey: string(message.Key),
			Payload:      cloneBytes(message.Payload),
		},
	); err != nil {
		return fmt.Errorf("发布死信消息失败：%w", err)
	}

	return nil
}

func (r *ConsumerRouter) handle(ctx context.Context, message eventbus.IncomingEvent) error {
	inboxState, err := r.claim(ctx, message)
	if err != nil {
		if eventbus.IsNonRetryable(err) {
			if publishErr := r.publishDeadLetter(ctx, message); publishErr != nil {
				return publishErr
			}
			return nil
		}

		log.Printf("消息抢占失败：event_id=%s，error=%v", message.EventID, err)
		return err
	}

	// 已经 completed 或 dead，不需要重复执行业务逻辑。
	if !inboxState.claimed {
		return nil
	}

	handlerErr := r.execute(ctx, message)
	if handlerErr != nil {
		if eventbus.IsNonRetryable(handlerErr) ||
			(r.maxInboxRetries > 0 && inboxState.retryCount >= r.maxInboxRetries) {
			if err := r.publishDeadLetter(ctx, message); err != nil {
				return err
			}
			if err := r.markDead(ctx, message, inboxState.lockToken, handlerErr.Error()); err != nil {
				return fmt.Errorf("标记 Inbox 死信状态失败：%w", err)
			}
			log.Printf(
				"消息进入死信队列：event_id=%s，retry_count=%d，error=%v",
				message.EventID,
				inboxState.retryCount,
				handlerErr,
			)
			return nil
		}

		if err := r.markRetry(ctx, message, inboxState.lockToken, handlerErr.Error()); err != nil {
			return fmt.Errorf("释放 Inbox 重试租约失败：%w", err)
		}

		log.Printf(
			"消息处理失败，等待 Kafka 重新投递：event_id=%s，error=%v",
			message.EventID,
			handlerErr,
		)
		return handlerErr
	}

	if err := r.markCompleted(ctx, message, inboxState.lockToken); err != nil {
		return fmt.Errorf("标记 Inbox 完成状态失败：%w", err)
	}

	return nil
}

func (r *ConsumerRouter) markRetry(ctx context.Context, message eventbus.IncomingEvent, lockToken, lastError string) error {
	if r == nil || r.inbox == nil || message.EventID == "" || lockToken == "" {
		return nil
	}
	return r.inbox.MarkRetry(ctx, message.EventID, lockToken, lastError)
}

func (r *ConsumerRouter) markDead(ctx context.Context, message eventbus.IncomingEvent, lockToken, lastError string) error {
	if r == nil || r.inbox == nil || message.EventID == "" || lockToken == "" {
		return nil
	}
	return r.inbox.MarkDead(ctx, message.EventID, lockToken, lastError, time.Now())
}

func (r *ConsumerRouter) markCompleted(ctx context.Context, message eventbus.IncomingEvent, lockToken string) error {
	if r == nil || r.inbox == nil || message.EventID == "" || lockToken == "" {
		return nil
	}
	return r.inbox.MarkCompleted(ctx, message.EventID, lockToken, time.Now())
}

type saramaAdapter struct {
	router      *ConsumerRouter
	topicRouter *TopicRouter
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
	ctx := session.Context()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			eventID := eventIDFromHeaders(message.Headers)

			eventName, mapErr := h.topicRouter.EventFor(message.Topic)
			if mapErr != nil {
				return mapErr
			}

			incomingEvent := eventbus.IncomingEvent{
				EventID: eventID,
				Name:    eventName,
				// 复制数据，避免业务层继续持有 Sarama 内部消息切片。
				Key:     cloneBytes(message.Key),
				Payload: cloneBytes(message.Value),
			}

			if err := h.router.handle(ctx, incomingEvent); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
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
	group         sarama.ConsumerGroup
	topicRouter   *TopicRouter
	topics        []string
	handlerRouter *ConsumerRouter
	retryInterval time.Duration
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
	topicRouter *TopicRouter,
	config configs.KafkaConsumerConfig,
) (*ConsumerGroup, error) {
	if client == nil || client.ConsumerGroup == nil {
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
		group:         client.ConsumerGroup,
		topics:        validTopics,
		handlerRouter: router,
		topicRouter:   topicRouter,
		retryInterval: time.Duration(config.ConsumeRetryIntervalSecs) * time.Second,
	}
	return group, nil
}

func (c *ConsumerGroup) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	handler := saramaAdapter{
		router:      c.handlerRouter,
		topicRouter: c.topicRouter,
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
