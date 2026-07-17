package mq_handler

import (
	"IM_backend/internal/infrastructure/messaging/client/kafka"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
)

type ReadMessageAckNotifyHandler struct {
	delivery RealtimeDelivery
}

func NewReadMessageAckNotifyHandler(delivery RealtimeDelivery) kafka.ConsumerHandler {
	return &ReadMessageAckNotifyHandler{delivery: delivery}
}

func (h *ReadMessageAckNotifyHandler) Handle(_ context.Context, message kafka.ConsumerMessage) error {
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
	return h.delivery.DeliverToUser(protocol.EventReadMessageNotify, event.SenderId, envelope.Payload)
}
