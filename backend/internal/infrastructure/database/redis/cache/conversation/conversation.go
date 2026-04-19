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

func (c *ConversationCache) SetMembers(ctx context.Context, convId string, userIds []string, version int64) error {
	if len(userIds) == 0 {
		return nil
	}

	membersKey := ConversationMembersKey(convId)
	versionKey := ConversationMembersVerKey(convId)

	script := `
		for i = 1, #ARGV-1 do
			redis.call('SADD', KEYS[1], ARGV[i])
		end

		redis.call('SET', KEYS[2], ARGV[#ARGV])
		return 1
	`

	args := make([]interface{}, 0, len(userIds)+1)
	for _, uid := range userIds {
		args = append(args, uid)
	}
	args = append(args, version)

	_, err := c.rb.Eval(
		ctx,
		script,
		[]string{membersKey, versionKey},
		args...,
	).Result()

	return err
}

func (c *ConversationCache) updateMemberWithVersion(
	ctx context.Context,
	convId string,
	userId string,
	version int64,
	op string,
) error {

	membersKey := ConversationMembersKey(convId)
	versionKey := ConversationMembersVerKey(convId)

	script := `
		if ARGV[3] == "add" then
			redis.call('SADD', KEYS[1], ARGV[1])
		else
			redis.call('SREM', KEYS[1], ARGV[1])
		end
		redis.call('SET', KEYS[2], ARGV[2])
		return 1
	`

	_, err := c.rb.Eval(
		ctx,
		script,
		[]string{membersKey, versionKey},
		userId,
		version,
		op,
	).Result()

	return err
}

func (c *ConversationCache) AddMember(ctx context.Context, convId, userId string, version int64) error {
	return c.updateMemberWithVersion(ctx, convId, userId, version, "add")
}

func (c *ConversationCache) RemoveMember(ctx context.Context, convId, userId string, version int64) error {
	return c.updateMemberWithVersion(ctx, convId, userId, version, "remove")
}

func (c *ConversationCache) GetMembersVersion(ctx context.Context, convId string) (int64, error) {
	key := ConversationMembersVerKey(convId)
	r, err := c.rb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return r, nil
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
