package jwt

import "github.com/golang-jwt/jwt/v4"

type Claims struct {
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}
