package handler

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	realtimeport "IM_backend/internal/application/ports/realtime"
	"IM_backend/internal/shared/protocol"
	"context"
)

type FriendRequestHandler struct {
	delivery realtimeport.UserDelivery
}

func NewFriendRequestHandler(delivery realtimeport.UserDelivery) eventbus.Handler {
	return &FriendRequestHandler{delivery: delivery}
}

func (handler *FriendRequestHandler) Handle(_ context.Context, message eventbus.IncomingEvent) error {
	envelope, err := decodeEnvelope(message.Payload)
	if err != nil {
		return eventbus.NonRetryable(err)
	}

	return handler.delivery.DeliverToUser(
		protocol.EventFriendRequestCreated,
		envelope.To,
		envelope.Payload,
	)
}
