package friend

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func FriendRelationKey(userID, friendID string) string {
	return cachekey.FriendRelation(userID, friendID)
}
