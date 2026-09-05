package message

import (
	messagecache "IM_backend/internal/application/ports/persistence/cache/message"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"encoding/json"
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
	senderID,
	clientMsgID string,
	entry messagecache.DedupEntry,
	ttl time.Duration,
) (bool, error) {
	value, err := json.Marshal(entry)
	if err != nil {
		return false, err
	}
	return c.store.SetNXString(ctx, MessageDedupKey(senderID, clientMsgID), string(value), ttl)
}

func (c *MessageCache) GetDedupEntry(ctx context.Context, senderID, clientMsgID string) (messagecache.DedupEntry, error) {
	value, err := c.store.GetString(ctx, MessageDedupKey(senderID, clientMsgID))
	if err != nil || value == "" {
		return messagecache.DedupEntry{}, err
	}

	var entry messagecache.DedupEntry
	if err := json.Unmarshal([]byte(value), &entry); err != nil {
		return messagecache.DedupEntry{}, err
	}
	return entry, nil
}
