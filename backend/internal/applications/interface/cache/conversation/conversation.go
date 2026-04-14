package conversation_cache_interface

import "context"

type ConversationCacheInterface interface {
	IsMember(ctx context.Context, convId, userId string) (bool, error)
	SetMembers(ctx context.Context, convId string, userIds []string) error
}
