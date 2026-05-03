package friend_cache

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type FriendCache struct {
	rb *redis.Client
}

func NewFriendCache(rb *redis.Client) *FriendCache {
	return &FriendCache{
		rb: rb,
	}
}

func (c *FriendCache) IsFriend(ctx context.Context, userId, friendUserId string) (bool, error) {
	key := FriendSetKey(userId)
	return c.rb.SIsMember(ctx, key, friendUserId).Result()
}

func (c *FriendCache) AddFriend(ctx context.Context, userId, friendUserId string) error {
	pipe := c.rb.Pipeline()

	key1 := FriendSetKey(userId)
	key2 := FriendSetKey(friendUserId)

	pipe.SAdd(ctx, key1, friendUserId)
	pipe.SAdd(ctx, key2, userId)

	_, err := pipe.Exec(ctx)
	return err
}

func (c *FriendCache) GetFriends(ctx context.Context, userId string) ([]string, error) {
	key := FriendSetKey(userId)
	return c.rb.SMembers(ctx, key).Result()
}

func (c *FriendCache) SetFriends(ctx context.Context, userId string, friendIds []string) error {
	if len(friendIds) == 0 {
		return nil
	}

	key := FriendSetKey(userId)

	values := make([]interface{}, 0, len(friendIds))
	for _, id := range friendIds {
		values = append(values, id)
	}

	return c.rb.SAdd(ctx, key, values...).Err()
}

func (c *FriendCache) RemoveFriend(ctx context.Context, userId, friendUserId string) error {
	pipe := c.rb.Pipeline()

	key1 := FriendSetKey(userId)
	key2 := FriendSetKey(friendUserId)

	pipe.SRem(ctx, key1, friendUserId)
	pipe.SRem(ctx, key2, userId)

	_, err := pipe.Exec(ctx)
	return err
}

func (c *FriendCache) DeleteUserFriends(ctx context.Context, userId []string) error {
	keys := make([]string, 0, len(userId))

	for _, u := range userId {
		keys = append(keys, FriendSetKey(u))
	}

	return c.rb.Del(ctx, keys...).Err()
}
