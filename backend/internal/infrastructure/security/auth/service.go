package auth

import (
	"IM_backend/internal/infrastructure/security/jwt"
	"time"
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

func (a *TokenIssuer) IssueToken(userId string) (string, string, error) {
	accessToken, err := jwt.GenerateToken(userId,
		a.options.Secret,
		a.options.AccessTokenTTL,
	)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := jwt.GenerateToken(
		userId,
		a.options.Secret,
		a.options.RefreshTokenTTL,
	)

	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}
