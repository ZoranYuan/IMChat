package conversation

import (
	conversationentity "IM_backend/internal/domain/conversation/entity"
	"context"
)

type UserConversationRepository interface {
	CreateUserConversation(ctx context.Context, conversation *conversationentity.UserConversation) error
	GetUsersByConversationID(ctx context.Context, conversationID string) ([]string, error)
	BatchUpdateSyncSeq(ctx context.Context, conversations []*conversationentity.UserConversation) error
	GetUserConversation(ctx context.Context, userId, conversationId string) (*conversationentity.UserConversation, error)
	UpdateSyncSeq(ctx context.Context, conversation *conversationentity.UserConversation) error
	UpdateReadSeq(ctx context.Context, conversation *conversationentity.UserConversation) error
	ListByUser(ctx context.Context, userId string) ([]*conversationentity.UserConversation, error)
	WithTx(tx any) UserConversationRepository
}
