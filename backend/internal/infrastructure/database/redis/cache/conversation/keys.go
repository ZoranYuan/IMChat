package conversation_cache

func ConversationMembersKey(conversation string) string {
	return "conversation:members:" + conversation
}
