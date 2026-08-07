package kafka

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	"context"

	"github.com/IBM/sarama"
)

type Producer struct {
	client *Client
	router *TopicRouter
}

func NewProducer(c *Client, router *TopicRouter) *Producer {
	return &Producer{client: c, router: router}
}

func (p *Producer) Publish(ctx context.Context, event eventbus.IntegrationEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	targetTopic, err := p.router.TopicFor(event.Name)
	if err != nil {
		return err
	}

	message := &sarama.ProducerMessage{
		Topic: targetTopic,

		Key: sarama.StringEncoder(event.PartitionKey),

		Value: sarama.ByteEncoder(event.Payload),
	}

	// 将 eventId 加入到 ProducerMessage 的Header 中
	if event.EventID != "" {
		message.Headers = []sarama.RecordHeader{{
			Key:   []byte("event_id"),
			Value: []byte(event.EventID),
		}}
	}

	_, _, err = p.client.Producer.SendMessage(message)

	return err
}
