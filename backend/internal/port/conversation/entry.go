package conversation_port

import "context"

type ConversationCacheInterface interface {
	AddMember(ctx context.Context, convID, userID string) error
	SetMembers(ctx context.Context, convID string, userIDs []string) error
	Delete(ctx context.Context, convID string) error
	GetMembers(ctx context.Context, conversation string) ([]string, error)
	RemoveMember(ctx context.Context, roomId, userId string) error
}
