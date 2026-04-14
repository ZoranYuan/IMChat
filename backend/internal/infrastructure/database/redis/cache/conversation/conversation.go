package conversation_cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type ConversationCache struct {
	rb *redis.Client
}

func NewConversationCache(rb *redis.Client) *ConversationCache {
	return &ConversationCache{
		rb: rb,
	}
}

func (c *ConversationCache) IsMember(ctx context.Context, convId, userId string) (bool, error) {
	key := ConversationMembersKey(convId)
	return c.rb.SIsMember(ctx, key, userId).Result()
}

func (c *ConversationCache) AddMember(ctx context.Context, convId, userId string) error {
	key := ConversationMembersKey(convId)
	return c.rb.SAdd(ctx, key, userId).Err()
}

func (c *ConversationCache) SetMembers(ctx context.Context, convId string, userIds []string) error {
	key := ConversationMembersKey(convId)

	if len(userIds) == 0 {
		return nil
	}

	values := make([]interface{}, 0, len(userIds))
	for _, uid := range userIds {
		values = append(values, uid)
	}

	return c.rb.SAdd(ctx, key, values...).Err()
}

func (rc *ConversationCache) RemoveMember(ctx context.Context, roomId, userId string) error {
	key := ConversationMembersKey(roomId)
	return rc.rb.SRem(ctx, key, userId).Err()
}

func (rc *ConversationCache) GetMembers(ctx context.Context, conversationId string) ([]string, error) {
	key := ConversationMembersKey(conversationId)
	return rc.rb.SMembers(ctx, key).Result()
}

func (c *ConversationCache) DeleteConversation(ctx context.Context, convId string) error {
	key := ConversationMembersKey(convId)
	return c.rb.Del(ctx, key).Err()
}
