package auth

import (
	"context"
	"time"
)

type AuthCache interface {
	SetRefreshSession(ctx context.Context, token string, session RefreshSession, expire time.Duration) error
	GetRefreshSession(ctx context.Context, token string) (RefreshSession, bool, error)
	RotateRefreshSession(
		ctx context.Context,
		oldToken string,
		newToken string,
		session RefreshSession,
		expire time.Duration,
	) (bool, error)
	DeleteRefreshSession(ctx context.Context, token string) error
}

type RefreshSession struct {
	UserID    string `json:"userId"`
	SessionID string `json:"sessionId"`
}
