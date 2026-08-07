package auth

import (
	authport "IM_backend/internal/application/ports/persistence/cache/auth"
	cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"
	"IM_backend/internal/infrastructure/persistence/redis/cache/shared"
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const rotateRefreshSessionScript = `
	local current = redis.call("GET", KEYS[1])
	if not current then
		return 0
	end
	redis.call("DEL", KEYS[1])
	redis.call("SET", KEYS[2], ARGV[1], "PX", ARGV[2])
	return 1
`

const revokedSessionValue = "revoked"

type authCache struct {
	store *shared.Store
}

func NewAuthCache(client *redis.Client) *authCache {
	return &authCache{
		store: shared.NewStore(client),
	}
}

func (ac *authCache) SetRefreshSession(
	ctx context.Context,
	token string,
	session authport.RefreshSession,
	expire time.Duration,
) error {
	return ac.store.SetJSON(ctx, RefreshTokenKey(token), session, expire)
}

func (ac *authCache) GetRefreshSession(
	ctx context.Context,
	token string,
) (authport.RefreshSession, bool, error) {
	var session authport.RefreshSession
	found, err := ac.store.GetJSON(ctx, RefreshTokenKey(token), &session)
	return session, found, err
}

func (ac *authCache) RotateRefreshSession(
	ctx context.Context,
	oldToken string,
	newToken string,
	session authport.RefreshSession,
	expire time.Duration,
) (bool, error) {
	data, err := json.Marshal(session)
	if err != nil {
		return false, err
	}

	result, err := ac.store.Client().Eval(
		ctx,
		rotateRefreshSessionScript,
		[]string{RefreshTokenKey(oldToken), RefreshTokenKey(newToken)},
		string(data),
		expire.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (ac *authCache) DeleteRefreshSession(
	ctx context.Context,
	token string,
) error {
	return ac.store.Del(ctx, RefreshTokenKey(token))
}

func (ac *authCache) RevokeSession(
	ctx context.Context,
	sessionID string,
	expire time.Duration,
) error {
	if sessionID == "" {
		return nil
	}
	return ac.store.SetString(ctx, cachekey.AuthRevokedSession(sessionID), revokedSessionValue, expire)
}

func (ac *authCache) IsSessionRevoked(
	ctx context.Context,
	sessionID string,
) (bool, error) {
	if sessionID == "" {
		return false, nil
	}
	value, err := ac.store.GetString(ctx, cachekey.AuthRevokedSession(sessionID))
	return value != "", err
}
