package friend

import (
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"

	"github.com/redis/go-redis/v9"
)

type FriendCache struct {
	store *shared.Store
}

func NewFriendCache(rb *redis.Client) *FriendCache {
	return &FriendCache{
		store: shared.NewStore(rb),
	}
}

func (c *FriendCache) IsFriend(ctx context.Context, userId, friendUserId string) (bool, error) {
	key := FriendSetKey(userId)
	return c.store.SIsMember(ctx, key, friendUserId)
}

func (c *FriendCache) AddFriend(ctx context.Context, userId, friendUserId string) error {
	key1 := FriendSetKey(userId)
	key2 := FriendSetKey(friendUserId)
	return c.store.SAddPair(ctx, key1, friendUserId, key2, userId)
}

func (c *FriendCache) GetFriends(ctx context.Context, userId string) ([]string, error) {
	key := FriendSetKey(userId)
	return c.store.SMembers(ctx, key)
}

func (c *FriendCache) SetFriends(ctx context.Context, userId string, friendIds []string) error {
	if len(friendIds) == 0 {
		return nil
	}

	key := FriendSetKey(userId)

	return c.store.SAddStrings(ctx, key, friendIds...)
}

func (c *FriendCache) RemoveFriend(ctx context.Context, userId, friendUserId string) error {
	key1 := FriendSetKey(userId)
	key2 := FriendSetKey(friendUserId)
	return c.store.SRemPair(ctx, key1, friendUserId, key2, userId)
}

func (c *FriendCache) DeleteUserFriends(ctx context.Context, userIds []string) error {
	keys := make([]string, 0, len(userIds))

	for _, userId := range userIds {
		keys = append(keys, FriendSetKey(userId))
	}

	return c.store.Del(ctx, keys...)
}
