package mq_client

import "context"

type Client interface {
	SendMessage(context.Context, string, string, []byte) error
}
