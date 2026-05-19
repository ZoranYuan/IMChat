package auth

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func AccessTokenKey(token string) string {
	return cachekey.AuthAccessToken(token)
}

func AccessUserKey(userId string) string {
	return cachekey.AuthAccessUser(userId)
}

func RefreshUserKey(userId string) string {
	return cachekey.AuthRefreshUser(userId)
}

func RefreshTokenKey(token string) string {
	return cachekey.AuthRefreshToken(token)
}
