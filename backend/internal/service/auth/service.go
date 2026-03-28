package service_auth

import (
	"IM_backend/configs"
	"IM_backend/internal/infrastructure/pkg/jwt"
	"fmt"
	"time"
)

type authService struct {
	config configs.Config
}

func NewAuthService(config configs.Config) *authService {
	return &authService{
		config: config,
	}
}

func (a *authService) IssueToken(userId string) (string, string, error) {
	fmt.Println("debug")
	accessToken, err := jwt.GenerateToken(userId,
		a.config.JWT.Secret,
		time.Duration(a.config.JWT.AccessExpireHours)*time.Hour,
	)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := jwt.GenerateToken(
		userId,
		a.config.JWT.Secret,
		time.Duration(a.config.JWT.AccessExpireHours)*time.Hour,
	)

	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}
