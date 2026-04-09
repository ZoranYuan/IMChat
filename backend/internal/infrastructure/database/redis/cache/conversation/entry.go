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

func (c *ConversationCache) AddMember(ctx context.Context, convID, userID string) error {
	key := ConversationMembersKey(convID)
	return c.rb.SAdd(ctx, key, userID).Err()
}

// 设置会话，用于重建缓存
func (c *ConversationCache) SetMembers(ctx context.Context, convID string, userIDs []string) error {
	key := ConversationMembersKey(convID)

	if len(userIDs) == 0 {
		return nil
	}

	values := make([]interface{}, 0, len(userIDs))
	for _, uid := range userIDs {
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

// 删除会话
func (c *ConversationCache) Delete(ctx context.Context, convID string) error {
	key := ConversationMembersKey(convID)
	return c.rb.Del(ctx, key).Err()
}
