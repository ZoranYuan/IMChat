package auth

import (
	"context"
	"time"
)

type AuthCache interface {
	SetAccessToken(ctx context.Context, token, userID string, expire time.Duration) error
	SetRefreshToken(ctx context.Context, token, userID string, expire time.Duration) error
	DeleteRefreshToken(ctx context.Context, token string) error
	GetUserIDByAccessToken(ctx context.Context, token string) (string, error)
	GetUserIDByRefreshToken(ctx context.Context, token string) (string, error)
}
