package message

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func MessageDedupKey(clientMsgID string) string {
	return cachekey.MessageDedup(clientMsgID)
}
