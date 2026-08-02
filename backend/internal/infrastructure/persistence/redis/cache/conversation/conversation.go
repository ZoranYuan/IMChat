package conversation

import (
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var _ convcache.ConversationCache = (*ConversationCache)(nil)

const incrIfExistsScript = `
local value = redis.call("GET", KEYS[1])
if not value then
	return 0
end

if tonumber(value) == -1 then
	return -1
end

return redis.call("INCR", KEYS[1])
`

const recoverConvLatestSeqScript = `
local dbSeq = tonumber(ARGV[1]) or 0
local value = redis.call("GET", KEYS[1])
local cacheSeq = tonumber(value)

if not cacheSeq or cacheSeq < dbSeq then
	redis.call("SET", KEYS[1], dbSeq)
end

return 1
`

const (
	conversationNotFoundSeq = int64(-1)
	conversationNotFoundTTL = time.Minute
)

type ConversationCache struct {
	store *shared.Store
}

func NewConversationCache(rb *redis.Client) *ConversationCache {
	return &ConversationCache{store: shared.NewStore(rb)}
}

func (c *ConversationCache) RecoverConvLatestSeq(
	ctx context.Context,
	conversationID string,
	dbLatestSeq int64,
) error {
	_, err := c.store.Eval(
		ctx,
		recoverConvLatestSeqScript,
		[]string{
			ConversationSeqKey(conversationID),
		},
		dbLatestSeq,
	)
	return err
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
	if seq == conversationNotFoundSeq {
		return 0, convcache.ErrConversationNotFound
	}

	return seq, nil
}

func (c *ConversationCache) GetConvLatestSeq(ctx context.Context, convID string) (int64, error) {
	seq, err := c.store.GetInt64(ctx, ConversationSeqKey(convID))
	if errors.Is(err, redis.Nil) {
		return seq, conversationentity.ErrConversationNotCreated
	}
	if err == nil && seq == conversationNotFoundSeq {
		return 0, convcache.ErrConversationNotFound
	}
	return seq, err
}

func (c *ConversationCache) MarkConversationNotFound(
	ctx context.Context,
	conversationID string,
) error {
	err := c.store.Client().SetArgs(
		ctx,
		ConversationSeqKey(conversationID),
		conversationNotFoundSeq,
		redis.SetArgs{
			Mode: "NX",
			TTL:  conversationNotFoundTTL,
		},
	).Err()
	return err
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
