package kafka

import (
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	messageentity "IM_backend/internal/domain/message/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
	"golang.org/x/sync/singleflight"
)

type GroupHandler struct {
	dispatch                   ClientDispatcher
	conversationCache          convcache.ConversationCache
	workerPool                 *WorkerPool
	sf                         singleflight.Group
	roomRepository             roomrepo.RoomRepository
	userConversationRepository messagerepo.UserConversationRepository
}

func NewGroupHandler(dispatcher ClientDispatcher,
	roomRepository roomrepo.RoomRepository,
	userConversationRepository messagerepo.UserConversationRepository,
	conversationCache convcache.ConversationCache,
) *GroupHandler {
	return &GroupHandler{
		dispatch:                   dispatcher,
		conversationCache:          conversationCache,
		userConversationRepository: userConversationRepository,
		roomRepository:             roomRepository,
		workerPool:                 newWorkerPool(100, 1000),
	}
}

func (h *GroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *GroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *GroupHandler) getConvMembers(ctx context.Context, conversationId string, roomId string) ([]string, error) {
	members, cacheVersion, err := h.conversationCache.GetMembersWithVersion(ctx, conversationId)
	if err == nil && cacheVersion > 0 && len(members) > 0 {
		return members, nil
	}

	v, err, _ := h.sf.Do(conversationId, func() (any, error) {
		members, cacheVersion, err := h.conversationCache.GetMembersWithVersion(ctx, conversationId)
		if err == nil && cacheVersion > 0 && len(members) > 0 {
			return members, nil
		}

		members, err = h.userConversationRepository.GetUsersByConversationID(ctx, conversationId)
		if err != nil {
			return nil, err
		}
		room, err := h.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
		if err != nil {
			return nil, err
		}

		version := room.Version
		if err := h.conversationCache.SetMembers(ctx, conversationId, members, version); err != nil {
			// TODO: 异步补偿（MQ / retry）
		}

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

		uc := messageentity.BuildUserConversation(
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

			userConvs := make([]*messageentity.UserConversation, 0, memberCount)

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
					messageentity.BuildUserConversation(
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

func (h *GroupHandler) handleConversationSyncSeq(
	ctx context.Context,
	envelope protocol.Envelope,
) error {
	var event protocol.ConversationSyncSeqEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return err
	}

	if len(event.Items) == 0 {
		return nil
	}

	userConvs := make([]*messageentity.UserConversation, 0, len(event.Items))
	for _, item := range event.Items {
		if item.UserId == "" || item.ConversationId == "" {
			continue
		}
		userConvs = append(userConvs, messageentity.BuildUserConversation(
			item.UserId,
			item.ConversationId,
			0,
			item.LatestSeq,
		))
	}

	return h.userConversationRepository.BatchUpdateSyncSeq(ctx, userConvs)
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

		var err error
		switch claim.Topic() {
		case protocol.EventTypeMessage:
			err = h.handleMessage(session.Context(), msg.Topic, string(msg.Key), envelope)
		case protocol.EventMessageReadAck:
			err = h.handleMessageReadAck(msg.Topic, envelope)
		case protocol.EventConversationSyncSeq:
			err = h.handleConversationSyncSeq(session.Context(), envelope)
		default:
			log.Println("unknown topic")
		}

		if err != nil {
			log.Printf("consume topic %s failed: %v", claim.Topic(), err)
		}
		session.MarkMessage(msg, "")
	}

	return nil
}
