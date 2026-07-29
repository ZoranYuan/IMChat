package handler

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	realtimeport "IM_backend/internal/application/ports/realtime"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
)

type ReadHandler struct {
	delivery realtimeport.Delivery
}

func NewReadHandler(delivery realtimeport.Delivery) eventbus.Handler {
	return &ReadHandler{delivery: delivery}
}

func (handler *ReadHandler) Handle(ctx context.Context, message eventbus.IncomingEvent) error {
	envelope, err := decodeEnvelope(message.Payload)
	if err != nil {
		return eventbus.NonRetryable(err)
	}

	var event protocol.MessageReadCommittedEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return eventbus.NonRetryable(err)
	}
	if len(event.NotifyUserIds) == 0 {
		return nil
	}

	notify := protocol.MessageReadAckEvent{
		UserId:         event.ReaderId,
		ConversationId: event.ConversationId,
		LastReadSeq:    event.LastReadSeq,
		ConvType:       event.ConvType,
		Avatar:         event.Avatar,
	}
	payload, err := json.Marshal(notify)
	if err != nil {
		return err
	}

	for _, userID := range event.NotifyUserIds {
		if err := handler.delivery.DeliverToUser(
			protocol.EventReadMessageNotify,
			userID,
			payload,
		); err != nil {
			return err
		}
	}
	return nil
}
