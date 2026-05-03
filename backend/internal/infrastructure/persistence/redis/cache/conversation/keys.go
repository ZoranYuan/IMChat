package conversation

func ConversationMembersKey(conversation string) string {
	return "conversation:members:" + conversation
}

func ConversationSeqKeys(convId string) string {
	return "conversation:seq:" + convId
}

func ConversationMembersVerKey(conversation string) string {
	return "conversation:members:ver:" + conversation
}
