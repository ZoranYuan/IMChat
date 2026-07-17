package conversation

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func ConversationSeqKey(conversationID string) string {
	return cachekey.ConversationSeq(conversationID)
}
