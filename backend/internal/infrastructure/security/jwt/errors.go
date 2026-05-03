package jwt

import "errors"

var (
	errInvalidToken = errors.New("invalid token")
	errExpiredToken = errors.New("token expired")
)
