package eventbus

import (
	"context"
	"errors"
)

type IntegrationEvent struct {
	EventID      string
	Name         string
	PartitionKey string
	Payload      []byte
}

type Publisher interface {
	Publish(ctx context.Context, event IntegrationEvent) error
}

type IncomingEvent struct {
	EventID string
	Name    string
	Key     []byte
	Payload []byte
}

type Handler interface {
	Handle(ctx context.Context, event IncomingEvent) error
}

type NonRetryableError struct {
	err error
}

func (e *NonRetryableError) Error() string { return e.err.Error() }
func (e *NonRetryableError) Unwrap() error { return e.err }

func NonRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &NonRetryableError{err: err}
}

func IsNonRetryable(err error) bool {
	var target *NonRetryableError
	return errors.As(err, &target)
}
