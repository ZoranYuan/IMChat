package handler

import (
	eventbus "IM_backend/internal/application/ports/eventbus"
	roomapp "IM_backend/internal/application/room"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
)

type RoomMemberChangedHandler struct {
	application *roomapp.RoomApplication
}

func NewRoomMemberChangedHandler(application *roomapp.RoomApplication) eventbus.Handler {
	return &RoomMemberChangedHandler{application: application}
}

func (handler *RoomMemberChangedHandler) Handle(ctx context.Context, message eventbus.IncomingEvent) error {
	var event protocol.RoomMemberChangedEvent
	if err := json.Unmarshal(message.Payload, &event); err != nil {
		return eventbus.NonRetryable(err)
	}
	if event.RoomID == "" || event.UserID == "" || event.Version <= 0 {
		return eventbus.NonRetryable(roomapp.ErrInvalidMemberChangedEvent)
	}
	return handler.application.ApplyMemberChanged(ctx, event)
}
