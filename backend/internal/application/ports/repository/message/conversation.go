package message_repository_interface

import (
	message_entity "IM_backend/internal/domain/message/entity"
	"context"
)

type ConversationRepository interface {
	CreateConversation(ctx context.Context, conv *message_entity.Conversation) error

	GetConversationSeq(ctx context.Context, conversationID string) (int64, error)

	GetByID(ctx context.Context, conversationId string) (*message_entity.Conversation, error)

	Upsert(
		ctx context.Context,
		domain *message_entity.Conversation,
	) error

	ListByIDs(
		ctx context.Context,
		ids []string,
	) ([]*message_entity.Conversation, error)

	WithTx(tx any) ConversationRepository
}
