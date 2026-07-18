package mq_handler

import (
	conversationapp "IM_backend/internal/application/conversation"
	"IM_backend/internal/infrastructure/messaging/client/kafka"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
)

type ConversationSyncHandler struct {
	service ConversationSyncService
}

func NewConversationSyncHandler(service ConversationSyncService) kafka.ConsumerHandler {
	return &ConversationSyncHandler{service: service}
}

func (h *ConversationSyncHandler) Handle(ctx context.Context, message kafka.ConsumerMessage) error {
	envelope, err := decodeEnvelope(message.Value)
	if err != nil {
		return err
	}

	var event protocol.ConversationSyncSeqEvent
	if err := json.Unmarshal(envelope.Payload, &event); err != nil {
		return err
	}

	items := make([]conversationapp.SyncSeq, 0, len(event.Items))
	for _, item := range event.Items {
		items = append(items, conversationapp.SyncSeq{
			UserId:         item.UserId,
			ConversationId: item.ConversationId,
			LatestSeq:      item.LatestSeq,
		})
	}
	return h.service.SyncLatestSequences(ctx, items)
}
