package kafka

import (
	"context"

	"github.com/IBM/sarama"
)

type Producer struct {
	client *Client
	topic  string
}

func NewProducer(c *Client, topic string) *Producer {
	return &Producer{
		client: c,
		topic:  topic,
	}
}

func (p *Producer) SendMessage(ctx context.Context, topic string, key string, payload []byte) error {
	targetTopic := p.topic
	if topic != "" {
		targetTopic = topic
	}

	_, _, err := p.client.Producer.SendMessage(&sarama.ProducerMessage{
		Topic: targetTopic,

		Key: sarama.StringEncoder(key),

		Value: sarama.ByteEncoder(payload),
	})

	return err
}
