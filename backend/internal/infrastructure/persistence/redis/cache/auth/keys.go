package auth

import (
	cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"
	"crypto/sha256"
	"encoding/hex"
)

func RefreshTokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return cachekey.AuthRefreshToken(hex.EncodeToString(sum[:]))
}
