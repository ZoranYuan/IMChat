package kafka

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
)

type ConsumerMessage struct {
	Topic string
	Key   []byte
	Value []byte
}

const defaultConsumeRetryInterval = time.Second

var (
	ErrHandlerNotFound = errors.New("消息处理器不存在")
	ErrInvalidClient   = errors.New("Kafka 客户端无效")
	ErrEmptyTopics     = errors.New("Kafka 主题不能为空")
)

type ConsumerHandler interface {
	Handle(context.Context, ConsumerMessage) error
}

type ConsumerRouter struct {
	handlers map[string]ConsumerHandler
}

func NewConsumerRouter(handlers map[string]ConsumerHandler) *ConsumerRouter {
	registered := make(map[string]ConsumerHandler, len(handlers))
	for topic, handler := range handlers {
		if topic == "" || handler == nil {
			continue
		}

		registered[topic] = handler
	}
	return &ConsumerRouter{handlers: registered}
}

func (r *ConsumerRouter) handle(ctx context.Context, message ConsumerMessage) error {
	if r == nil {
		return ErrHandlerNotFound
	}

	handler, ok := r.handlers[message.Topic]
	if !ok {
		return ErrHandlerNotFound
	}
	return handler.Handle(ctx, message)
}

type saramaAdapter struct {
	router *ConsumerRouter
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
				ConsumerMessage{
					Topic: message.Topic,

					// 复制数据，避免业务层继续持有 Sarama 内部消息切片。
					Key:   cloneBytes(message.Key),
					Value: cloneBytes(message.Value),
				})

			if err != nil {
				// 不调用 MarkMessage，当前 offset 不会被当前处理流程提交。
				return fmt.Errorf(
					"处理 Kafka 消息失败：topic=%s partition=%d offset=%d：%w",
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

func NewConsumerGroup(client *Client, topics []string, router *ConsumerRouter) (*ConsumerGroup, error) {
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

	return &ConsumerGroup{
		group:         client.Consumer,
		topics:        validTopics,
		handlerRouter: router,
		retryInterval: defaultConsumeRetryInterval,
	}, nil
}

func (c *ConsumerGroup) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	handler := saramaAdapter{c.handlerRouter}
	for ctx.Err() == nil {
		if err := c.group.Consume(ctx, c.topics, handler); err != nil {
			log.Printf(
				"消费 Kafka 主题失败，topics=%v，retry_after=%s，err=%v",
				c.topics,
				c.retryInterval,
				err,
			)

			// 失败的任务当占有时间结束时，会重新被 Outbox 抢占
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
