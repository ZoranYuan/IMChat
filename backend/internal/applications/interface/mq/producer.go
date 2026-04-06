package mq_interface

import "context"

type Producer interface {
	SendMessage(ctx context.Context, topic string, key string, payload Payload) error
}
