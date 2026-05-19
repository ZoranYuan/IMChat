package conversation

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func ConversationMembersKey(conversation string) string {
	return cachekey.ConversationMembers(conversation)
}

func ConversationSeqKeys(convId string) string {
	return cachekey.ConversationSeq(convId)
}

func ConversationMembersVerKey(conversation string) string {
	return cachekey.ConversationMembersVersion(conversation)
}
