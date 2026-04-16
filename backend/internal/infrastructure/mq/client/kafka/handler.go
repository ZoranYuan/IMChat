package kafka

import (
	message_entity "IM_backend/internal/domain/message/entity"
	message_repository_interface "IM_backend/internal/domain/message/repository"
	"IM_backend/internal/protocol"
	"encoding/json"
	"log"

	"github.com/IBM/sarama"
)

type GroupHandler struct {
	dispacth                   Dispatch
	userConversationRepository message_repository_interface.UserConversationRepositoryInterface
}

func NewGroupHandler(dispacth Dispatch, userConversationRepository message_repository_interface.UserConversationRepositoryInterface) *GroupHandler {
	return &GroupHandler{
		dispacth:                   dispacth,
		userConversationRepository: userConversationRepository,
	}
}

func (h *GroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *GroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
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
			return err
		}

		to := envelope.To

		switch msg.Topic {
		case protocol.EventTypeMessage:
			if err := h.dispacth.SendToClient(msg.Topic, to, envelope.Payload); err != nil {
				log.Println("failed to send message, ", err)
			}

			var message protocol.MessageEvent

			if err := json.Unmarshal(envelope.Payload, &message); err != nil {
				log.Println("failed to parse payload")
				return err
			}

			uc := message_entity.BuildUserConversation(
				to,
				message.ConversationId,
				message.MessageId,
				0,
				message.Seq,
			)

			h.userConversationRepository.UpdateSyncSeq(session.Context(), uc)
		case protocol.EventTypeHistoryMessageReadAck:
			if err := h.dispacth.SendToClient(msg.Topic, to, envelope.Payload); err != nil {
				log.Println("failed to ack history message, ", err)
			}
		}

		session.MarkMessage(msg, "")
	}

	return nil
}
