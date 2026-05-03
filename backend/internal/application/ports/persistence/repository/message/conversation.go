package message

import (
	messageentity "IM_backend/internal/domain/message/entity"
	"context"
)

type ConversationRepository interface {
	CreateConversation(ctx context.Context, conv *messageentity.Conversation) error

	GetConversationSeq(ctx context.Context, conversationID string) (int64, error)

	GetByID(ctx context.Context, conversationId string) (*messageentity.Conversation, error)

	Upsert(
		ctx context.Context,
		domain *messageentity.Conversation,
	) error

	ListByIDs(
		ctx context.Context,
		ids []string,
	) ([]*messageentity.Conversation, error)

	WithTx(tx any) ConversationRepository
}
