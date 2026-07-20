package conversation

import (
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var _ convcache.ConversationCache = (*ConversationCache)(nil)

const incrIfExistsScript = `
local value = redis.call("GET", KEYS[1])
if not value then
	return 0
end
return redis.call("INCR", KEYS[1])
`

const recoverAndIncrScript = `
local dbSeq = tonumber(ARGV[1]) or 0
local value = redis.call("GET", KEYS[1])

if not value then
	redis.call("SET", KEYS[1], dbSeq)
	return redis.call("INCR", KEYS[1])
end

local cacheSeq = tonumber(value) or 0
if cacheSeq < dbSeq then
	redis.call("SET", KEYS[1], dbSeq)
end

return redis.call("INCR", KEYS[1])
`

type ConversationCache struct {
	store *shared.Store
}

func NewConversationCache(rb *redis.Client) *ConversationCache {
	return &ConversationCache{store: shared.NewStore(rb)}
}

func (c *ConversationCache) IncrConvLatestSeq(ctx context.Context, convID string) (int64, error) {
	result, err := c.store.Eval(ctx, incrIfExistsScript, []string{ConversationSeqKey(convID)})
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, conversationentity.ErrConversationNotCreated
		}
		return 0, err
	}

	seq, err := toInt64(result)
	if err != nil {
		return 0, err
	}
	if seq == 0 {
		return 0, conversationentity.ErrConversationNotCreated
	}

	return seq, nil
}

func (c *ConversationCache) RecoverConvLatestSeqAndIncr(
	ctx context.Context,
	convID string,
	dbLatestSeq int64,
) (int64, error) {
	result, err := c.store.Eval(
		ctx,
		recoverAndIncrScript,
		[]string{ConversationSeqKey(convID)},
		dbLatestSeq,
	)
	if err != nil {
		return 0, err
	}

	return toInt64(result)
}

func (c *ConversationCache) GetConvLatestSeq(ctx context.Context, convID string) (int64, error) {
	seq, err := c.store.GetInt64(ctx, ConversationSeqKey(convID))
	if errors.Is(err, redis.Nil) {
		return seq, conversationentity.ErrConversationNotCreated
	}
	return seq, err
}

func (c *ConversationCache) SetConvSeq(ctx context.Context, convID string, seq int64) error {
	return c.store.SetNXInt64(ctx, ConversationSeqKey(convID), seq)
}

func toInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("未知的类型 %T", value)
	}
}
