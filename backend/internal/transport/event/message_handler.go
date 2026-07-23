package event

import (
	messageapp "IM_backend/internal/application/message"
	eventbus "IM_backend/internal/application/ports/eventbus"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	realtimeport "IM_backend/internal/application/ports/realtime"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"log"

	"golang.org/x/sync/singleflight"
)

type MessageHandler struct {
	delivery           realtimeport.Delivery
	roomMemberCache    roomcache.RoomMemberCache
	roomRepository     roomrepo.RoomRepository
	roomUserRepository roomrepo.RoomUserRepository
	singleflight       singleflight.Group
}

func NewMessageHandler(
	delivery realtimeport.Delivery,
	roomRepository roomrepo.RoomRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	roomMemberCache roomcache.RoomMemberCache,
) eventbus.Handler {
	return &MessageHandler{
		delivery:           delivery,
		roomMemberCache:    roomMemberCache,
		roomRepository:     roomRepository,
		roomUserRepository: roomUserRepository,
	}
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

	err = handler.deliverMessage(ctx, string(message.Name), string(message.Key), envelope, event)
	if errors.Is(err, messageapp.ErrUnknownConversationType) {
		return eventbus.NonRetryable(err)
	}
	return err
}

func (handler *MessageHandler) deliverMessage(
	ctx context.Context,
	eventType string,
	conversationID string,
	envelope protocol.Envelope,
	event protocol.MessageEvent,
) error {
	switch event.ConvType {
	case protocol.PrivateChat:
		return handler.delivery.DeliverToUser(eventType, envelope.To, envelope.Payload)

	case protocol.RoomChat:
		members, err := handler.roomMembers(ctx, conversationID)
		if err != nil {
			return err
		}
		for _, userID := range members {
			if userID == envelope.From {
				continue
			}
			if err := handler.delivery.DeliverToUser(eventType, envelope.To, envelope.Payload); err != nil {
				return err
			}
		}
		return nil

	default:
		return messageapp.ErrUnknownConversationType
	}
}

func (handler *MessageHandler) roomMembers(ctx context.Context, roomID string) ([]string, error) {
	members, cached, err := handler.roomMemberCache.GetMemberIDs(ctx, roomID)
	if err == nil && cached {
		return members, nil
	}

	value, err, _ := handler.singleflight.Do(roomID, func() (any, error) {
		members, cached, err := handler.roomMemberCache.GetMemberIDs(ctx, roomID)
		if err == nil && cached {
			return members, nil
		}
		if _, err := handler.roomRepository.FindActiveRoom(roomID, int(roomvo.Activate)); err != nil {
			return nil, err
		}
		members, err = handler.roomUserRepository.ListActiveUserIDs(roomID)
		if err != nil {
			return nil, err
		}
		if err := handler.roomMemberCache.SetMemberIDs(ctx, roomID, members); err != nil {
			log.Printf("警告：房间 %s 的成员缓存预热失败：%v", roomID, err)
		}
		return members, nil
	})
	if err != nil {
		return nil, err
	}
	return value.([]string), nil
}

func decodeEnvelope(data []byte) (protocol.Envelope, error) {
	var envelope protocol.Envelope
	err := json.Unmarshal(data, &envelope)
	return envelope, err
}
