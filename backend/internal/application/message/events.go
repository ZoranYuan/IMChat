package message

type MessageSendEvent struct {
	MessageID      string
	ConversationID string

	SenderID   string
	ReceiverID string

	Seq  int64
	Type int

	Content string

	SendTime int64
}
