package auth

import (
	"IM_backend/internal/infrastructure/security/jwt"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
)

type Options struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type TokenIssuer struct {
	options Options
}

func NewTokenIssuer(options Options) *TokenIssuer {
	return &TokenIssuer{options: options}
}

func (a *TokenIssuer) IssueToken(userId string) (string, string, string, error) {
	sessionID := uuid.NewString()
	accessToken, err := jwt.GenerateAccessToken(
		userId,
		sessionID,
		a.options.Secret,
		a.options.AccessTokenTTL,
	)
	if err != nil {
		return "", "", "", err
	}

	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return "", "", "", err
	}

	refreshToken := base64.RawURLEncoding.EncodeToString(refreshBytes)
	return accessToken, refreshToken, sessionID, nil
}
