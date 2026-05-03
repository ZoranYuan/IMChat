package kafka

import (
	conversation_port "IM_backend/internal/application/ports/cache/conversation"
	message_repository_interface "IM_backend/internal/application/ports/repository/message"
	room_repository_interface "IM_backend/internal/application/ports/repository/room"
	message_entity "IM_backend/internal/domain/message/entity"
	room_valueobject "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/persistence/redis/cache/local"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
	"golang.org/x/sync/singleflight"
)

type GroupHandler struct {
	dispatch                   ClientDispatcher
	conversationCache          conversation_port.ConversationCache
	localConversationCache     *local.ConversationVersionCache
	workerPool                 *WorkerPool
	sf                         singleflight.Group
	roomRepository             room_repository_interface.RoomRepository
	userConversationRepository message_repository_interface.UserConversationRepository
}

func NewGroupHandler(dispatcher ClientDispatcher,
	roomRepository room_repository_interface.RoomRepository,
	userConversationRepository message_repository_interface.UserConversationRepository,
	conversationCache conversation_port.ConversationCache,
	localConversationCache *local.ConversationVersionCache,
) *GroupHandler {
	return &GroupHandler{
		dispatch:                   dispatcher,
		conversationCache:          conversationCache,
		userConversationRepository: userConversationRepository,
		roomRepository:             roomRepository,
		workerPool:                 newWorkerPool(100, 1000),
		localConversationCache:     localConversationCache,
	}
}

func (h *GroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *GroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *GroupHandler) getConvMembers(ctx context.Context, conversationId string, roomId string) ([]string, error) {
	localVersion, ok := h.localConversationCache.GetVersion(conversationId)

	members, cacheVersion, err := h.conversationCache.GetMembersWithVersion(ctx, conversationId)
	if ok && localVersion == cacheVersion {
		return members, nil
	}

	v, err, _ := h.sf.Do(conversationId, func() (any, error) {
		// double check
		localVersion, ok := h.localConversationCache.GetVersion(conversationId)

		members, cacheVersion, err := h.conversationCache.GetMembersWithVersion(ctx, conversationId)
		if err == nil {
			if ok && localVersion == cacheVersion && len(members) > 0 {
				return members, nil
			}
		}

		members, err = h.userConversationRepository.GetUsersByConversationID(ctx, conversationId)
		if err != nil {
			return nil, err
		}
		room, err := h.roomRepository.FindActiveRoom(roomId, int(room_valueobject.Activate))
		if err != nil {
			return nil, err
		}

		version := room.Version
		if err := h.conversationCache.SetMembers(ctx, conversationId, members, version); err != nil {
			// TODO: 异步补偿（MQ / retry）
		}

		// 7️⃣ 再写本地（L1）
		h.localConversationCache.SetVersion(conversationId, version)
		return members, nil
	})

	if err != nil {
		return nil, err
	}

	return v.([]string), nil
}

func (h *GroupHandler) handleMessage(
	ctx context.Context,
	topic, conversationId string,
	envelope protocol.Envelope,
) error {
	var event protocol.MessageEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return err
	}

	switch event.ConvType {
	case protocol.PrivateChat:
		if err := h.dispatch.SendToClient(topic, envelope.To, envelope.Payload); err != nil {
			// TODO:补偿
		}

		uc := message_entity.BuildUserConversation(
			envelope.To,
			conversationId,
			0,
			event.Seq,
		)

		if err := h.userConversationRepository.UpdateSyncSeq(ctx, uc); err != nil {
			// TODO:补偿
		}

	case protocol.RoomChat:
		members, err := h.getConvMembers(ctx, conversationId, conversationId)
		if err != nil {
			// TODO: 补偿
		}

		memberCount := len(members)
		if memberCount <= 100 {
			payload := envelope.Payload

			userConvs := make([]*message_entity.UserConversation, 0, memberCount)

			for _, uid := range members {
				id := uid
				h.workerPool.submit(func() {
					if err = h.dispatch.SendToClient(
						topic,
						id,
						payload,
					); err != nil {
						// TODO:补偿
					}
				})

				userConvs = append(userConvs,
					message_entity.BuildUserConversation(
						id,
						conversationId,
						0,
						event.Seq,
					),
				)
			}

			if err := h.userConversationRepository.BatchUpdateSyncSeq(ctx, userConvs); err != nil {
				// TODO:补偿

			}
		} else {
			// TODO: 以某种方式通知前端群聊有新消息，让前端主动去拉取消息
		}
	default:
		return ErrUnknownConversationType
	}

	return nil
}

func (h *GroupHandler) handleMessageReadAck(
	topic string,
	envelope protocol.Envelope,
) error {
	var event protocol.MessageReadAckEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return err
	}

	if err := h.dispatch.SendToClient(topic, envelope.To, envelope.Payload); err != nil {
		// TODO: 补偿
	}

	return nil
}

func (h *GroupHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim,
) error {

	for msg := range claim.Messages() {
		var envelope protocol.Envelope

		if err := json.Unmarshal(msg.Value, &envelope); err != nil {
			session.MarkMessage(msg, "")
			continue
		}

		switch claim.Topic() {
		case protocol.EventTypeMessage:
			h.handleMessage(session.Context(), msg.Topic, string(msg.Key), envelope)
		case protocol.EventMessageReadAck:
			h.handleMessageReadAck(msg.Topic, envelope)
		default:
			log.Println("unknow topic")
		}
	}

	return nil
}
