package message

type MessageSendEvent struct {
	MessageId      string
	ConversationId string

	SendId string
	RecvId string

	Seq  int64
	Type int

	// TODO content 不一定是内容，还能使其他的，所以可能要引入一个 ContentType 字段
	Content string

	SendTime int64
}
