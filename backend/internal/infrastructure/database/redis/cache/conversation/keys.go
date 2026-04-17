package conversation_cache

func ConversationMembersKey(conversation string) string {
	return "conversation:members:" + conversation
}

func ConversationSeqKeys(convId string) string {
	return "conversation:seq:" + convId
}
