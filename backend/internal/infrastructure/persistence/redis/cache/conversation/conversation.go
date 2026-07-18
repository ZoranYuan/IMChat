package conversation

import (
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

var _ convcache.ConversationCache = (*ConversationCache)(nil)

type ConversationCache struct {
	store *shared.Store
}

func NewConversationCache(rb *redis.Client) *ConversationCache {
	return &ConversationCache{store: shared.NewStore(rb)}
}

func (c *ConversationCache) IncrConvLatestSeq(ctx context.Context, convID string) (int64, error) {
	seq, err := c.store.Incr(ctx, ConversationSeqKey(convID))
	if errors.Is(err, redis.Nil) {
		return seq, conversationentity.ErrConversationNotCreated
	}
	return seq, err
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
