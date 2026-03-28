package auth_cache_interface

import (
	"context"
	"time"
)

type AuthCacheInterface interface {
	SetAccessToken(ctx context.Context, userId, token string, expire time.Duration) error
	SetRefreshToken(ctx context.Context, userId, token string, expire time.Duration) error
	DeleteRefreshToken(ctx context.Context, token string) error
	GetUserIdByAccessToken(ctx context.Context, token string) (string, error)
	GetUserIdByRefreshToken(ctx context.Context, token string) (string, error)
}
