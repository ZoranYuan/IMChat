package mq

import (
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
)

type ConversationSyncHandler struct {
	repository messagerepo.UserConversationRepository
}

func NewConversationSyncHandler(repository messagerepo.UserConversationRepository) *ConversationSyncHandler {
	return &ConversationSyncHandler{repository: repository}
}

func (h *ConversationSyncHandler) Handle(ctx context.Context, message ConsumerMessage) error {
	envelope, err := decodeEnvelope(message.Value)
	if err != nil {
		return err
	}

	var event protocol.ConversationSyncSeqEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return err
	}

	userConversations := make([]*messageentity.UserConversation, 0, len(event.Items))
	for _, item := range event.Items {
		if item.UserId == "" || item.ConversationId == "" {
			continue
		}
		userConversations = append(userConversations, messageentity.BuildUserConversation(
			item.UserId,
			item.ConversationId,
			0,
			item.LatestSeq,
		))
	}
	if len(userConversations) == 0 {
		return nil
	}
	return h.repository.BatchUpdateSyncSeq(ctx, userConversations)
}
