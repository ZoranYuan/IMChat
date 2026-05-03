package kafka

import (
	"context"
	"log"
)

type ClientDispatcher interface {
	SendToClient(string, string, []byte) error
}

type Consumer struct {
	client       *Client
	topic        []string
	groupID      string
	groupHandler *GroupHandler
}

func NewConsumer(c *Client, topic []string, groupID string, groupHandler *GroupHandler) *Consumer {
	return &Consumer{
		client:       c,
		topic:        topic,
		groupID:      groupID,
		groupHandler: groupHandler,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	for {
		err := c.client.Consumer.Consume(
			ctx,
			c.topic,
			c.groupHandler,
		)

		if err != nil {
			log.Println("consume error:", err)
		}

		// context cancel 退出
		if ctx.Err() != nil {
			log.Println("consum error:", ctx.Err())
			return ctx.Err()
		}
	}
}
