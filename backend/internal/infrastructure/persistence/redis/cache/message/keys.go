package message

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func MessageDedupKey(sendID, clientMsgID string) string {
	return cachekey.MessageDedup(sendID, clientMsgID)
}
