package mq_handler

import (
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	messageentity "IM_backend/internal/domain/message/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/messaging/client/kafka"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"log"

	"golang.org/x/sync/singleflight"
)

var ErrUnknownConversationType = errors.New("unknown conversation type")

type RealtimeDelivery interface {
	DeliverToUser(eventType, userID string, payload []byte) error
}

type messageSendHandler struct {
	delivery                   RealtimeDelivery
	roomMemberCache            roomcache.RoomMemberCache
	roomRepository             roomrepo.RoomRepository
	roomUserRepository         roomrepo.RoomUserRepository
	userConversationRepository messagerepo.UserConversationRepository
	sf                         singleflight.Group
}

func (h *messageSendHandler) Handle(ctx context.Context, message kafka.ConsumerMessage) error {
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
		for _, userId := range members {
			if userId == envelope.From {
				continue
			}

			if err := h.delivery.DeliverToUser(message.Topic, userId, envelope.Payload); err != nil {
				return err
			}
		}

		userConversations := make([]*messageentity.UserConversation, 0, len(members))
		for _, userId := range members {
			if userId == envelope.From {
				continue
			}

			userConversations = append(userConversations, messageentity.BuildUserConversation(
				userId,
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

func (h *messageSendHandler) roomMembers(ctx context.Context, conversationID string) ([]string, error) {
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

func NewSendMessagehandler(
	delivery RealtimeDelivery,
	roomRepository roomrepo.RoomRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	userConversationRepository messagerepo.UserConversationRepository,
	roomMemberCache roomcache.RoomMemberCache,
) kafka.ConsumerHandler {
	return &messageSendHandler{
		delivery:                   delivery,
		roomMemberCache:            roomMemberCache,
		roomRepository:             roomRepository,
		roomUserRepository:         roomUserRepository,
		userConversationRepository: userConversationRepository,
	}
}
