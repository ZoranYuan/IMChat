package conversation

import (
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

type ConversationCache struct {
	store *shared.Store
}

func NewConversationCache(rb *redis.Client) *ConversationCache {
	return &ConversationCache{
		store: shared.NewStore(rb),
	}
}

func (c *ConversationCache) IsMemberWithVersion(ctx context.Context, convId, userId string) (bool, int64, error) {
	script := `
		local isMember = redis.call("SISMEMBER", KEYS[1], ARGV[1])
		local version = redis.call("GET", KEYS[2])
		return {isMember, version}
	`

	memberKey := ConversationMembersKey(convId)
	versionKey := ConversationMembersVerKey(convId)

	res, err := c.store.Eval(
		ctx,
		script,
		[]string{memberKey, versionKey},
		userId,
	)
	if err != nil {
		return false, 0, err
	}

	data, ok := res.([]interface{})
	if !ok || len(data) != 2 {
		return false, 0, fmt.Errorf("invalid lua result")
	}

	isMember := data[0].(int64) == 1

	var version int64
	if data[1] != nil {
		s := fmt.Sprint(data[1])
		version, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return false, 0, err
		}
	}

	return isMember, version, nil
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

	_, err := c.store.Eval(
		ctx,
		script,
		[]string{membersKey, versionKey},
		args...,
	)

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

	_, err := c.store.Eval(
		ctx,
		script,
		[]string{membersKey, versionKey},
		userId,
		version,
		op,
	)

	return err
}

func (c *ConversationCache) AddMember(ctx context.Context, convId, userId string, version int64) error {
	return c.updateMemberWithVersion(ctx, convId, userId, version, "add")
}

func (c *ConversationCache) RemoveMember(ctx context.Context, convId, userId string, version int64) error {
	return c.updateMemberWithVersion(ctx, convId, userId, version, "remove")
}

func (c *ConversationCache) GetMembersWithVersion(
	ctx context.Context,
	convId string,
) ([]string, int64, error) {

	memberKey := ConversationMembersKey(convId)
	versionKey := ConversationMembersVerKey(convId)

	script := `
		local members = redis.call("SMEMBERS", KEYS[1])
		local version = redis.call("GET", KEYS[2])
		return {members, version}
	`

	res, err := c.store.Eval(
		ctx,
		script,
		[]string{memberKey, versionKey},
	)
	if err != nil {
		return nil, 0, err
	}

	data, ok := res.([]interface{})
	if !ok || len(data) != 2 {
		return nil, 0, fmt.Errorf("invalid lua result")
	}

	rawMembers, ok := data[0].([]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid members type")
	}

	members := make([]string, 0, len(rawMembers))
	for _, v := range rawMembers {
		if v == nil {
			continue
		}

		switch val := v.(type) {
		case []byte:
			members = append(members, string(val))
		case string:
			members = append(members, val)
		default:
			return nil, 0, fmt.Errorf("unexpected member type %T", v)
		}
	}

	var version int64
	if data[1] != nil {
		s := fmt.Sprint(data[1])
		version, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, 0, err
		}
	}

	return members, version, nil
}

func (c *ConversationCache) DeleteConversation(ctx context.Context, convId string) error {
	key := ConversationMembersKey(convId)
	return c.store.Del(ctx, key)
}

func (mc *ConversationCache) IncrConvLatestSeq(ctx context.Context, convId string) (int64, error) {
	key := ConversationSeqKeys(convId)

	r, err := mc.store.Incr(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return r, messageentity.ErrConversationNotCreated
		}

		return r, err
	}

	return r, nil
}

func (mc *ConversationCache) GetConvLatestSeq(ctx context.Context, convId string) (int64, error) {
	key := ConversationSeqKeys(convId)
	r, err := mc.store.GetInt64(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return r, messageentity.ErrConversationNotCreated
		}

		return r, err
	}

	return r, nil
}

func (mc *ConversationCache) SetConvSeq(ctx context.Context, convId string, seq int64) error {
	key := ConversationSeqKeys(convId)
	return mc.store.SetNXInt64(ctx, key, seq)
}
