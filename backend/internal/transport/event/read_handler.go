package event

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
)

type ReadHandler struct {
	delivery MessageDelivery
}

func NewReadHandler(delivery MessageDelivery) eventbus.Handler {
	return &ReadHandler{delivery: delivery}
}

func (handler *ReadHandler) Handle(ctx context.Context, message eventbus.IncomingEvent) error {
	envelope, err := decodeEnvelope(message.Payload)
	if err != nil {
		return eventbus.NonRetryable(err)
	}

	var event protocol.MessageReadAckEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return eventbus.NonRetryable(err)
	}
	return handler.delivery.DeliverReadNotification(ctx, event, envelope.Payload)
}
