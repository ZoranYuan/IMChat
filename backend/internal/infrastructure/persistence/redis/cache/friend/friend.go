package friend

import (
	friendcache "IM_backend/internal/application/ports/persistence/cache/friend"
	friendvo "IM_backend/internal/domain/friend/value_object"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	friendRelationTTL         = 6 * time.Hour
	friendRelationNegativeTTL = 2 * time.Minute
)

type relationEntry struct {
	Found  bool `json:"found"`
	Status int  `json:"status,omitempty"`
}

type FriendCache struct {
	store *shared.Store
}

var _ friendcache.FriendCache = (*FriendCache)(nil)

func NewFriendCache(rb *redis.Client) *FriendCache {
	return &FriendCache{
		store: shared.NewStore(rb),
	}
}

func (c *FriendCache) GetRelation(
	ctx context.Context,
	userID, friendID string,
) (*friendcache.RelationState, bool, error) {
	key := FriendRelationKey(userID, friendID)
	data, err := c.store.Client().Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var entry relationEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		_ = c.store.Del(ctx, key)
		return nil, false, fmt.Errorf("解析好友关系缓存失败：%w", err)
	}
	if !entry.Found {
		return nil, true, nil
	}
	return &friendcache.RelationState{Status: friendvo.Status(entry.Status)}, true, nil
}

func (c *FriendCache) SetRelation(
	ctx context.Context,
	userID, friendID string,
	state *friendcache.RelationState,
) error {
	if state == nil {
		return fmt.Errorf("好友关系状态不能为空")
	}
	return c.setRelationEntry(ctx, userID, friendID, relationEntry{
		Found:  true,
		Status: int(state.Status),
	}, friendRelationTTL)
}

func (c *FriendCache) SetRelationNotFound(ctx context.Context, userID, friendID string) error {
	return c.setRelationEntry(ctx, userID, friendID, relationEntry{}, friendRelationNegativeTTL)
}

func (c *FriendCache) DeleteRelation(ctx context.Context, userID, friendID string) error {
	return c.store.Del(ctx, FriendRelationKey(userID, friendID))
}

func (c *FriendCache) setRelationEntry(
	ctx context.Context,
	userID, friendID string,
	entry relationEntry,
	ttl time.Duration,
) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return c.store.Client().Set(ctx, FriendRelationKey(userID, friendID), data, ttl).Err()
}
