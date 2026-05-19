package user

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func UserInfoKey(userId string) string {
	return cachekey.UserInfo(userId)
}
