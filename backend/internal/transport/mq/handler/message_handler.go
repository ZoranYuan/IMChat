package handler

import (
	messageapp "IM_backend/internal/application/message"
	eventbus "IM_backend/internal/application/ports/eventbus"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
)

type MessageDelivery interface {
	Deliver(
		ctx context.Context,
		eventType string,
		conversationID string,
		envelope protocol.Envelope,
		event protocol.MessageEvent,
	) error
}

type messageDeliveryCloser interface {
	Close(ctx context.Context)
}

type MessageHandler struct {
	delivery MessageDelivery
}

func NewMessageHandler(delivery MessageDelivery) *MessageHandler {
	return &MessageHandler{
		delivery: delivery,
	}
}

func (handler *MessageHandler) Close(ctx context.Context) {
	if closer, ok := handler.delivery.(messageDeliveryCloser); ok {
		closer.Close(ctx)
	}
}

func (handler *MessageHandler) Handle(ctx context.Context, message eventbus.IncomingEvent) error {
	envelope, err := decodeEnvelope(message.Payload)
	if err != nil {
		return eventbus.NonRetryable(err)
	}

	var messageEvent protocol.MessageEvent
	if err := json.Unmarshal(envelope.Payload, &messageEvent); err != nil {
		return eventbus.NonRetryable(err)
	}

	err = handler.delivery.Deliver(ctx, string(message.Name), string(message.Key), envelope, messageEvent)
	if errors.Is(err, messageapp.ErrUnknownConversationType) {
		return eventbus.NonRetryable(err)
	}
	return err
}

func decodeEnvelope(data []byte) (protocol.Envelope, error) {
	var envelope protocol.Envelope
	err := json.Unmarshal(data, &envelope)
	return envelope, err
}
