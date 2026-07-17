package message

import (
	messagecache "IM_backend/internal/application/ports/persistence/cache/message"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var _ messagecache.MessageCache = (*MessageCache)(nil)

type MessageCache struct {
	store *shared.Store
}

func NewMessageCache(client *redis.Client) *MessageCache {
	return &MessageCache{store: shared.NewStore(client)}
}

func (c *MessageCache) SetDedupEntry(
	ctx context.Context,
	clientMsgID, messageID string,
	ttl time.Duration,
) (bool, error) {
	return c.store.SetNXString(ctx, MessageDedupKey(clientMsgID), messageID, ttl)
}

func (c *MessageCache) GetDedupEntry(ctx context.Context, clientMsgID string) (string, error) {
	return c.store.GetString(ctx, MessageDedupKey(clientMsgID))
}
