package conversation_port

import "context"

type ConversationCacheInterface interface {
	IsMember(ctx context.Context, convId, userId string) (bool, error)
	SetMembers(ctx context.Context, convId string, userIds []string) error
	AddMember(ctx context.Context, convId, userId string) error
	DeleteConversation(ctx context.Context, convID string) error
	GetMembers(ctx context.Context, conversation string) ([]string, error)
	RemoveMember(ctx context.Context, roomId, userId string) error
}
