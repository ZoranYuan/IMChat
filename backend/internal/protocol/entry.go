package protocol

type Envelope struct {
	From    string
	To      string
	Payload []byte
}

type AckStatus string

const (
	AckStatusSending   AckStatus = "sending"
	AckStatusSent      AckStatus = "sent"
	AckStatusDelivered AckStatus = "delivered"
	AckStatusRead      AckStatus = "read"
	AckStatusFailed    AckStatus = "failed"
)

type AckEvent struct {
	ClientMsgId string    `json:"clientMsgId"`
	MessageId   string    `json:"messageId"`
	Status      AckStatus `json:"status"`
	SendTime    int64     `json:"sendTime"`
	ErrorMsg    string    `json:"errorMsg"`
}

type MessageEvent struct {
	MessageId      string `json:"messageId"`
	ConversationId string `json:"conversationId"`
	SendId         string `json:"sendId"`
	RecvId         string `json:"recvId"`
	Seq            int64  `json:"seq"`
	ConvType       int    `json:"convType"`
	CType          int    `json:"cType"`
	Content        string `json:"content"`
	SendTime       int64  `json:"sendTime"`
}
