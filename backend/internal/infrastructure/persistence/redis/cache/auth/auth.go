package auth_cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type authCache struct {
	client *redis.Client
}

func NewAuthCache(client *redis.Client) *authCache {
	return &authCache{
		client: client,
	}
}

func (ac *authCache) set(
	ctx context.Context,
	key string,
	value string,
	expire time.Duration,
) error {
	return ac.client.Set(ctx, key, value, expire).Err()
}

func (ac *authCache) get(
	ctx context.Context,
	key string,
) (string, error) {

	result, err := ac.client.Get(ctx, key).Result()

	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return result, nil
}

func (ac *authCache) del(
	ctx context.Context,
	key string,
) error {
	return ac.client.Del(ctx, key).Err()
}

func (ac *authCache) SetAccessToken(
	ctx context.Context,
	token string,
	userId string,
	expire time.Duration,
) error {
	return ac.set(ctx, AccessTokenKey(token), userId, expire)
}

func (ac *authCache) GetUserIDByAccessToken(
	ctx context.Context,
	token string,
) (string, error) {
	return ac.get(ctx, AccessTokenKey(token))
}

func (ac *authCache) SetRefreshToken(
	ctx context.Context,
	token string,
	userId string,
	expire time.Duration,
) error {
	return ac.set(ctx, RefreshTokenKey(token), userId, expire)
}

func (ac *authCache) GetUserIDByRefreshToken(
	ctx context.Context,
	token string,
) (string, error) {
	return ac.get(ctx, RefreshTokenKey(token))
}

func (ac *authCache) DeleteRefreshToken(
	ctx context.Context,
	token string,
) error {
	return ac.del(ctx, RefreshTokenKey(token))
}
