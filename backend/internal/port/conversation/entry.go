package conversation_port

import "context"

type ConversationCacheInterface interface {
	IsMember(ctx context.Context, convId, userId string) (bool, error)
	SetMembers(ctx context.Context, convId string, userIds []string) error
	DeleteConversation(ctx context.Context, convID string) error
	GetMembers(ctx context.Context, conversation string) ([]string, error)
	IncrConvLatestSeq(ctx context.Context, convId string) (int64, error)
	GetConvLatestSeq(ctx context.Context, convId string) (int64, error)
	SetConvSeq(ctx context.Context, convId string, seq int64) error
	AddMember(ctx context.Context, convId, userId string) error
	RemoveMember(ctx context.Context, convId, userId string) error
}
