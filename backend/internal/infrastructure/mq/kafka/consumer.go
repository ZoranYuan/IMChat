package kafka

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
)

const defaultConsumeRetryInterval = time.Second

var (
	ErrHandlerNotFound = errors.New("消息处理器不存在")
	ErrInvalidClient   = errors.New("消息队列客户端无效")
	ErrEmptyTopics     = errors.New("消息主题不能为空")
)

type ConsumerRouter struct {
	handlers map[string]eventbus.Handler
}

func NewConsumerRouter(handlers map[string]eventbus.Handler) *ConsumerRouter {
	registered := make(map[string]eventbus.Handler, len(handlers))
	for topic, handler := range handlers {
		if topic == "" || handler == nil {
			continue
		}

		registered[topic] = handler
	}
	return &ConsumerRouter{handlers: registered}
}

func (r *ConsumerRouter) handle(ctx context.Context, message eventbus.IncomingEvent) error {
	if r == nil {
		return ErrHandlerNotFound
	}

	handler, ok := r.handlers[message.Name]
	if !ok {
		return ErrHandlerNotFound
	}
	return handler.Handle(ctx, message)
}

type saramaAdapter struct {
	router              *ConsumerRouter
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

			err := h.router.handle(session.Context(),
				eventbus.IncomingEvent{
					Name: message.Topic,

					// 复制数据，避免业务层继续持有 Sarama 内部消息切片。
					Key:     cloneBytes(message.Key),
					Payload: cloneBytes(message.Value),
				})

			if err != nil {
				if eventbus.IsNonRetryable(err) && h.deadLetterPublisher != nil {
					if publishErr := h.deadLetterPublisher.Publish(
						session.Context(),
						eventbus.IntegrationEvent{
							Name:         message.Topic + h.deadLetterSuffix,
							PartitionKey: string(message.Key),
							Payload:      cloneBytes(message.Value),
						},
					); publishErr != nil {
						return fmt.Errorf("发布死信消息失败：%w", publishErr)
					}
					log.Printf(
						"毒消息已转入死信队列：主题=%s 分区=%d 偏移量=%d 错误=%v",
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
	topics              []string
	handlerRouter       *ConsumerRouter
	retryInterval       time.Duration
	deadLetterPublisher eventbus.Publisher
	deadLetterSuffix    string
}

type ConsumerGroupOption func(*ConsumerGroup)

func WithDeadLetterPublisher(publisher eventbus.Publisher) ConsumerGroupOption {
	return func(group *ConsumerGroup) {
		group.deadLetterPublisher = publisher
	}
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

func NewConsumerGroup(client *Client, topics []string, router *ConsumerRouter, options ...ConsumerGroupOption) (*ConsumerGroup, error) {
	if client == nil || client.Consumer == nil {
		return nil, ErrInvalidClient
	}

	validTopics := normalizeTopics(topics)
	if len(validTopics) == 0 {
		return nil, ErrEmptyTopics
	}

	if router == nil {
		router = NewConsumerRouter(nil)
	}

	group := &ConsumerGroup{
		group:            client.Consumer,
		topics:           validTopics,
		handlerRouter:    router,
		retryInterval:    defaultConsumeRetryInterval,
		deadLetterSuffix: ".dlq",
	}
	for _, option := range options {
		if option != nil {
			option(group)
		}
	}
	return group, nil
}

func (c *ConsumerGroup) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	handler := saramaAdapter{
		router:              c.handlerRouter,
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
	return context.Cause(ctx)
}

func (c *ConsumerGroup) Close() error {
	if c == nil || c.group == nil {
		return nil
	}

	return c.group.Close()
}
