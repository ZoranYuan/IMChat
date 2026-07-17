package mq

import (
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	messageentity "IM_backend/internal/domain/message/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"log"

	"golang.org/x/sync/singleflight"
)

var ErrUnknownConversationType = errors.New("unknown conversation type")

// RealtimeDelivery is defined by the messaging consumer which needs it.
type RealtimeDelivery interface {
	DeliverToUser(eventType, userID string, payload []byte) error
}

// MessagePushHandler consumes persisted message events and pushes them to
// online users. Kafka-specific lifecycle concerns stay outside this type.
type MessagePushHandler struct {
	delivery                   RealtimeDelivery
	roomMemberCache            roomcache.RoomMemberCache
	roomRepository             roomrepo.RoomRepository
	roomUserRepository         roomrepo.RoomUserRepository
	userConversationRepository messagerepo.UserConversationRepository
	sf                         singleflight.Group
}

func NewMessagePushHandler(
	delivery RealtimeDelivery,
	roomRepository roomrepo.RoomRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	userConversationRepository messagerepo.UserConversationRepository,
	roomMemberCache roomcache.RoomMemberCache,
) *MessagePushHandler {
	return &MessagePushHandler{
		delivery:                   delivery,
		roomMemberCache:            roomMemberCache,
		roomRepository:             roomRepository,
		roomUserRepository:         roomUserRepository,
		userConversationRepository: userConversationRepository,
	}
}

func (h *MessagePushHandler) Handle(ctx context.Context, message ConsumerMessage) error {
	envelope, err := decodeEnvelope(message.Value)
	if err != nil {
		return err
	}

	var event protocol.MessageEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return err
	}

	conversationID := string(message.Key)
	switch event.ConvType {
	case protocol.PrivateChat:
		if err := h.delivery.DeliverToUser(message.Topic, envelope.To, envelope.Payload); err != nil {
			return err
		}
		return h.userConversationRepository.UpdateSyncSeq(ctx, messageentity.BuildUserConversation(
			envelope.To,
			conversationID,
			0,
			event.Seq,
		))

	case protocol.RoomChat:
		members, err := h.roomMembers(ctx, conversationID)
		if err != nil {
			return err
		}
		for _, userID := range members {
			if err := h.delivery.DeliverToUser(message.Topic, userID, envelope.Payload); err != nil {
				return err
			}
		}

		userConversations := make([]*messageentity.UserConversation, 0, len(members))
		for _, userID := range members {
			userConversations = append(userConversations, messageentity.BuildUserConversation(
				userID,
				conversationID,
				0,
				event.Seq,
			))
		}
		return h.userConversationRepository.BatchUpdateSyncSeq(ctx, userConversations)

	default:
		return ErrUnknownConversationType
	}
}

func (h *MessagePushHandler) roomMembers(ctx context.Context, conversationID string) ([]string, error) {
	members, cached, err := h.roomMemberCache.GetMemberIDs(ctx, conversationID)
	if err == nil && cached {
		return members, nil
	}

	value, err, _ := h.sf.Do(conversationID, func() (any, error) {
		members, cached, err := h.roomMemberCache.GetMemberIDs(ctx, conversationID)
		if err == nil && cached {
			return members, nil
		}

		if _, err := h.roomRepository.FindActiveRoom(conversationID, int(roomvo.Activate)); err != nil {
			return nil, err
		}
		members, err = h.roomUserRepository.ListActiveUserIDs(conversationID)
		if err != nil {
			return nil, err
		}
		if err := h.roomMemberCache.SetMemberIDs(ctx, conversationID, members); err != nil {
			log.Printf("warn: cache warmup failed for conversation %s: %v", conversationID, err)
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
