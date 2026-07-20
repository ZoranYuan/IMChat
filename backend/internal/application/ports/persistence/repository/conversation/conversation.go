package conversation

import (
	conversationentity "IM_backend/internal/domain/conversation/entity"
	"context"
)

type ConversationRepository interface {
	CreateConversation(ctx context.Context, conversation *conversationentity.Conversation) error
	GetConversationSeq(ctx context.Context, conversationID string) (int64, error)
	GetByID(ctx context.Context, conversationID string) (*conversationentity.Conversation, error)
	UpdateLatestSequence(
		ctx context.Context,
		conversation *conversationentity.Conversation,
		updateLatestMessage bool,
	) error
	ListByIDs(ctx context.Context, ids []string) ([]*conversationentity.Conversation, error)
	WithTx(tx any) ConversationRepository
}
