package auth

import (
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type authCache struct {
	store *shared.Store
}

func NewAuthCache(client *redis.Client) *authCache {
	return &authCache{
		store: shared.NewStore(client),
	}
}

func (ac *authCache) set(
	ctx context.Context,
	key string,
	value string,
	expire time.Duration,
) error {
	return ac.store.SetString(ctx, key, value, expire)
}

func (ac *authCache) get(
	ctx context.Context,
	key string,
) (string, error) {
	return ac.store.GetString(ctx, key)
}

func (ac *authCache) del(
	ctx context.Context,
	key string,
) error {
	return ac.store.Del(ctx, key)
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
