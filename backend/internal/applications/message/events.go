package application_message

type MessageSendEvent struct {
	MessageId      string
	ConversationId string

	SendId string
	RecvId string

	Seq int64

	Type    int
	Content string

	SendTime int64
}
