package mq

import (
	"context"
	"errors"
)

var ErrHandlerNotFound = errors.New("message handler not found")

// ConsumerMessage is independent of a concrete message broker.
type ConsumerMessage struct {
	Topic string
	Key   []byte
	Value []byte
}

// ConsumerHandler is implemented by the business consumer of a topic.
type ConsumerHandler interface {
	Handle(context.Context, ConsumerMessage) error
}

type ConsumerHandlerFunc func(context.Context, ConsumerMessage) error

func (f ConsumerHandlerFunc) Handle(ctx context.Context, message ConsumerMessage) error {
	return f(ctx, message)
}

// ConsumerRouter maps broker topics to business handlers.
type ConsumerRouter struct {
	handlers map[string]ConsumerHandler
}

func NewConsumerRouter(handlers map[string]ConsumerHandler) *ConsumerRouter {
	registered := make(map[string]ConsumerHandler, len(handlers))
	for topic, handler := range handlers {
		if topic != "" && handler != nil {
			registered[topic] = handler
		}
	}
	return &ConsumerRouter{handlers: registered}
}

func (r *ConsumerRouter) Handle(ctx context.Context, message ConsumerMessage) error {
	handler, ok := r.handlers[message.Topic]
	if !ok {
		return ErrHandlerNotFound
	}
	return handler.Handle(ctx, message)
}
