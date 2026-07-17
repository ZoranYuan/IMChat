package protocol

import "encoding/json"

type Envelope struct {
	From    string
	To      string
	Payload []byte
}

type AckStatus string

const (
	AckStatusSent      AckStatus = "sent"
	AckStatusDelivered AckStatus = "delivered"
	AckStatusRead      AckStatus = "read"
	AckStatusFailed    AckStatus = "failed"
)

type EventType string

type ConvType int

const (
	PrivateChat ConvType = 1
	RoomChat    ConvType = 2
)

const (
	EventReadMessageAck      = "msg_read_ack"          // 前端发送给后端，表示当前用户已经读取了消息
	EventReadMessageNotify   = "msg_read_notify"       // 发送给前端，表示已经有用户读取消息
	EventTypeMsgAck          = "msg_ack"               // 发送给前端，表示已经接收到消息
	EventTypeSendMessage     = "msg"                   // 后端发送给前端，表示新的消息
	EventConversationSyncSeq = "conversation_sync_seq" // 表示消息的同步
)

type Event struct {
	Type EventType       `json:"type"`
	Data json.RawMessage `json:"data"`
}

type MessageReadAckEvent struct {
	UserId         string   `json:"userId"`
	ConversationId string   `json:"conversationId"`
	LastReadSeq    int64    `json:"lastReadSeq"`
	ConvType       ConvType `json:"convType"`
	SenderId       string   `json:"senderId"`
	Avatar         string   `json:"avatar,omitempty"`
}

type MessageAckEvent struct {
	ClientMsgId string    `json:"clientMsgId"`
	MessageId   string    `json:"messageId"`
	Status      AckStatus `json:"status"`
	Extra       string    `json:"extra"`
	SendTime    int64     `json:"sendTime"`
}

type MessageNotifyEvent struct {
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
	Seq            int64  `json:"seq"`
}

type MessageEvent struct {
	MessageId      string   `json:"messageId"`
	ConversationId string   `json:"conversationId"`
	SendId         string   `json:"sendId"`
	SenderUsername string   `json:"senderUsername"`
	RecvId         string   `json:"recvId"`
	Seq            int64    `json:"seq"`
	ConvType       ConvType `json:"convType"`
	CType          int      `json:"cType"`
	Content        string   `json:"content"`
	SendTime       int64    `json:"sendTime"`
	ClientMsgId    string   `json:"clientMsgId"`
	MediaURL       string   `json:"mediaUrl,omitempty"`
	ThumbURL       string   `json:"thumbUrl,omitempty"`
	FileId         string   `json:"fileId,omitempty"`
	ThumbFileId    string   `json:"thumbFileId,omitempty"`
	FileName       string   `json:"fileName,omitempty"`
	FileSize       int64    `json:"fileSize,omitempty"`
	Width          int      `json:"width,omitempty"`
	Height         int      `json:"height,omitempty"`
	DurationMs     *int64   `json:"durationMs,omitempty"`
	StickerId      string   `json:"stickerId,omitempty"`
	PackId         string   `json:"packId,omitempty"`
	HasVideoTime   bool     `json:"hasVideoTime,omitempty"`
	VideoTime      *int64   `json:"videoTime,omitempty"`
}

type ConversationSyncSeqItem struct {
	UserId         string `json:"userId"`
	ConversationId string `json:"conversationId"`
	LatestSeq      int64  `json:"latestSeq"`
}

type ConversationSyncSeqEvent struct {
	Items []ConversationSyncSeqItem `json:"items"`
}
