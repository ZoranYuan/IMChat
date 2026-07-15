package kafka

import (
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	messageentity "IM_backend/internal/domain/message/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	mq "IM_backend/internal/infrastructure/messaging"
	realtime "IM_backend/internal/infrastructure/realtime"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/IBM/sarama"
	"golang.org/x/sync/singleflight"
)

type GroupHandler struct {
	dispatch                   realtime.Gateway
	conversationCache          convcache.ConversationCache
	workerPool                 *WorkerPool
	sf                         singleflight.Group
	roomRepository             roomrepo.RoomRepository
	userConversationRepository messagerepo.UserConversationRepository
	batcher                    *mq.MessageBatcher
}

func NewGroupHandler(dispatcher realtime.Gateway,
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
		batcher:                    mq.NewMessageBatcher(dispatcher, 20, 50*time.Millisecond, 30*time.Second),
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
		room, roomErr := h.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
		if roomErr == nil && cacheVersion == room.Version {
			return members, nil
		}
	}

	v, err, _ := h.sf.Do(conversationId, func() (any, error) {
		members, cacheVersion, err := h.conversationCache.GetMembersWithVersion(ctx, conversationId)
		if err == nil && cacheVersion > 0 && len(members) > 0 {
			room, roomErr := h.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
			if roomErr == nil && cacheVersion == room.Version {
				return members, nil
			}
		}

		members, err = h.userConversationRepository.GetUsersByConversationID(ctx, conversationId)
		if err != nil {
			return nil, err
		}
		room, err := h.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate))
		if err != nil {
			return nil, err
		}

		if err := h.conversationCache.SetMembers(ctx, conversationId, members, room.Version); err != nil {
			// best-effort cache warmup; the DB state is already authoritative
			log.Printf("warn: cache warmup failed for conversation %s: %v", conversationId, err)
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
		if err := h.dispatch.DeliverToUser(topic, envelope.To, envelope.Payload); err != nil {
			return err
		}

		uc := messageentity.BuildUserConversation(
			envelope.To,
			conversationId,
			0,
			event.Seq,
		)

		if err := h.userConversationRepository.UpdateSyncSeq(ctx, uc); err != nil {
			return err
		}

	case protocol.RoomChat:
		members, err := h.getConvMembers(ctx, conversationId, conversationId)
		if err != nil {
			return err
		}

		// Push via batcher — messages are accumulated per-conversation
		// and flushed in batches, reducing per-message push overhead.
		if err := h.batcher.Add(topic, conversationId, members, event); err != nil {
			log.Printf("warn: batcher enqueue failed: %v", err)
			return err
		}

		userConvs := make([]*messageentity.UserConversation, 0, len(members))
		for _, uid := range members {
			userConvs = append(userConvs,
				messageentity.BuildUserConversation(uid, conversationId, 0, event.Seq),
			)
		}
		if err := h.userConversationRepository.BatchUpdateSyncSeq(ctx, userConvs); err != nil {
			return err
		}
	default:
		return ErrUnknownConversationType
	}

	return nil
}

func (h *GroupHandler) handleMessageReadAck(
	ctx context.Context,
	topic string,
	envelope protocol.Envelope,
) error {
	var event protocol.MessageReadAckEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return err
	}

	// 通知消息发送方：你的消息已被读取
	if event.SenderId != "" {
		return h.dispatchMessageReadNotify(event.SenderId, envelope.Payload)
	}

	return nil
}

func (h *GroupHandler) dispatchMessageReadNotify(senderId string, payload []byte) error {
	if senderId == "" {
		return nil
	}
	if err := h.dispatch.DeliverToUser(protocol.EventMessageReadNotify, senderId, payload); err != nil {
		return err
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
			err = h.handleMessageReadAck(session.Context(), msg.Topic, envelope)
		case protocol.EventConversationSyncSeq:
			err = h.handleConversationSyncSeq(session.Context(), envelope)
		default:
			log.Println("unknown topic")
		}

		if err != nil {
			log.Printf("consume topic %s failed: %v", claim.Topic(), err)
			continue
		}
		session.MarkMessage(msg, "")
	}

	return nil
}
