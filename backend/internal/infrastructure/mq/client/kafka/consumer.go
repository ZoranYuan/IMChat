package kafka

import (
	"context"
	"log"
)

type Dispatch interface {
	SendToClient(string, string, []byte) error
}

type Consumer struct {
	client   *Client
	topic    []string
	groupId  string
	dispacth Dispatch
}

func NewConsumer(c *Client, topic []string, groupId string, dispacth Dispatch) *Consumer {
	return &Consumer{
		client:   c,
		topic:    topic,
		groupId:  groupId,
		dispacth: dispacth,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	for {
		h := &groupHandler{
			dispacth: c.dispacth,
		}

		err := c.client.Consumer.Consume(
			ctx,
			c.topic,
			h,
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
