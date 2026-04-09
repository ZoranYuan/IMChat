package protocol

type Envelope struct {
	From    string
	To      string
	Payload []byte
}

type MessageEvent struct {
	MessageId      string
	ConversationId string
	SendId         string
	RecvId         string
	ConvType       int
	CType          int
	Content        string
	SendTime       int64
}
