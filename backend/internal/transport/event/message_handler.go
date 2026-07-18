package event

import (
	messageapp "IM_backend/internal/application/message"
	eventbus "IM_backend/internal/application/ports/eventbus"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
)

type MessageHandler struct {
	delivery MessageDelivery
}

func NewMessageHandler(delivery MessageDelivery) eventbus.Handler {
	return &MessageHandler{delivery: delivery}
}

func (handler *MessageHandler) Handle(ctx context.Context, message eventbus.IncomingEvent) error {
	envelope, err := decodeEnvelope(message.Payload)
	if err != nil {
		return eventbus.NonRetryable(err)
	}

	var event protocol.MessageEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return eventbus.NonRetryable(err)
	}

	err = handler.delivery.DeliverMessage(ctx, messageapp.DeliveryCommand{
		EventType:      message.Name,
		ConversationId: string(message.Key),
		FromUserId:     envelope.From,
		ToUserId:       envelope.To,
		Payload:        envelope.Payload,
		Message:        event,
	})
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
