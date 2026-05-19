package friend

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

const (
	FriendSetKeyPrefix = "im:user:" // user -> set(friendUserId)
)

func FriendSetKey(userId string) string {
	return cachekey.UserFriends(userId)
}
