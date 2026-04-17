package conversation_cache

import (
	message_entity "IM_backend/internal/domain/message/entity"
	"context"
	"errors"

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

func (c *ConversationCache) AddMember(ctx context.Context, convId, userId string) error {
	key := ConversationMembersKey(convId)
	return c.rb.SAdd(ctx, key, userId).Err()
}

func (c *ConversationCache) RemoveMember(ctx context.Context, convId, userId string) error {
	key := ConversationMembersKey(convId)
	return c.rb.SRem(ctx, key, userId).Err()
}

func (rc *ConversationCache) GetMembers(ctx context.Context, conversationId string) ([]string, error) {
	key := ConversationMembersKey(conversationId)
	return rc.rb.SMembers(ctx, key).Result()
}

func (c *ConversationCache) DeleteConversation(ctx context.Context, convId string) error {
	key := ConversationMembersKey(convId)
	return c.rb.Del(ctx, key).Err()
}

func (mc *ConversationCache) IncrConvLatestSeq(ctx context.Context, convId string) (int64, error) {
	key := ConversationSeqKeys(convId)

	r, err := mc.rb.Incr(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return r, message_entity.ErrConversationNotCreated
		}

		return r, err
	}

	return r, nil
}

func (mc *ConversationCache) GetConvLatestSeq(ctx context.Context, convId string) (int64, error) {
	key := ConversationSeqKeys(convId)
	r, err := mc.rb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return r, message_entity.ErrConversationNotCreated
		}

		return r, err
	}

	return r, nil
}

func (mc *ConversationCache) SetConvSeq(ctx context.Context, convId string, seq int64) error {
	key := ConversationSeqKeys(convId)
	return mc.rb.SetArgs(ctx, key, seq, redis.SetArgs{
		Mode: "NX",
	}).Err()
}
