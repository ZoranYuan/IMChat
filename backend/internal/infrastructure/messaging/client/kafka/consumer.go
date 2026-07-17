package kafka

import (
	mq "IM_backend/internal/infrastructure/messaging"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/IBM/sarama"
)

// ConsumerGroup runs Sarama sessions and delegates messages to a
// broker-independent business handler.
type ConsumerGroup struct {
	group   sarama.ConsumerGroup
	topics  []string
	handler mq.ConsumerHandler
}

func NewConsumerGroup(client *Client, topics []string, handler mq.ConsumerHandler) *ConsumerGroup {
	return &ConsumerGroup{
		group:   client.Consumer,
		topics:  append([]string(nil), topics...),
		handler: handler,
	}
}

func (c *ConsumerGroup) Start(ctx context.Context) error {
	adapter := saramaAdapter{handler: c.handler}
	for ctx.Err() == nil {
		if err := c.group.Consume(ctx, c.topics, adapter); err != nil {
			log.Printf("consume kafka topics failed: %v", err)
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return context.Cause(ctx)
			case <-timer.C:
			}
		}
	}
	return context.Cause(ctx)
}

type saramaAdapter struct {
	handler mq.ConsumerHandler
}

func (saramaAdapter) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (saramaAdapter) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h saramaAdapter) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {
	for message := range claim.Messages() {
		err := h.handler.Handle(session.Context(), mq.ConsumerMessage{
			Topic: message.Topic,
			Key:   append([]byte(nil), message.Key...),
			Value: append([]byte(nil), message.Value...),
		})
		if err != nil {
			return fmt.Errorf("handle topic %s partition %d offset %d: %w", message.Topic, message.Partition, message.Offset, err)
		}
		session.MarkMessage(message, "")
	}
	return nil
}
