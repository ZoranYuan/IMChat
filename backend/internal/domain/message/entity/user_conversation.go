package message_entity

type UserConversation struct {
	UserId         string
	ConversationId string

	LasetReadSeq int64
	IsMuted      bool
}

func BuildUserConversation(userId, conversationId string, seq int64) *UserConversation {
	return &UserConversation{
		UserId:         userId,
		ConversationId: conversationId,
		LasetReadSeq:   seq,
		IsMuted:        false,
	}
}
