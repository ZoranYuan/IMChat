package mq

import (
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
)

type ReadAckHandler struct {
	delivery RealtimeDelivery
}

func NewReadAckHandler(delivery RealtimeDelivery) *ReadAckHandler {
	return &ReadAckHandler{delivery: delivery}
}

func (h *ReadAckHandler) Handle(_ context.Context, message ConsumerMessage) error {
	envelope, err := decodeEnvelope(message.Value)
	if err != nil {
		return err
	}

	var event protocol.MessageReadAckEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return err
	}
	if event.SenderId == "" {
		return nil
	}
	return h.delivery.DeliverToUser(protocol.EventMessageReadNotify, event.SenderId, envelope.Payload)
}
