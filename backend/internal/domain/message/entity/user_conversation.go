package message_entity

type UserConversation struct {
	UserId         string
	ConversationId string

	LasetReadSeq int64

	IsMuted bool
}
