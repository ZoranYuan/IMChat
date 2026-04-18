package kafka

import (
	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	conversation_port "IM_backend/internal/port/conversation"
	"IM_backend/internal/protocol"
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/IBM/sarama"
)

type GroupHandler struct {
	dispatch                   Dispatch
	conversationCache          conversation_port.ConversationCacheInterface
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface
}

func NewGroupHandler(dispacth Dispatch,
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface,
	conversationCache conversation_port.ConversationCacheInterface,
) *GroupHandler {
	return &GroupHandler{
		dispatch:                   dispacth,
		conversationCache:          conversationCache,
		userConversationRepository: userConversationRepository,
	}
}

func (h *GroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *GroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
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
		members, err := h.conversationCache.GetMembers(ctx, conversationId)
		if err != nil {
			return err
		}

		memberCount := len(members)

		if memberCount <= 100 {
			// 小群，采取推模式
			payload := envelope.Payload

			userConvs := make([]*message_entity.UserConversation, 0, memberCount)

			for _, uid := range members {
				if err = h.dispatch.SendToClient(
					topic,
					uid,
					payload,
				); err != nil {
					// TODO:补偿
				}

				// 表示系统已经将消息进行了同步
				userConvs = append(userConvs,
					message_entity.BuildUserConversation(
						uid,
						conversationId,
						0,
						event.Seq,
					),
				)
			}

			// 批量更新 DB
			if err := h.userConversationRepository.BatchUpdateSyncSeq(ctx, userConvs); err != nil {
				// TODO:补偿

			}
		}
	default:
		return errors.New("unknow convType")
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
