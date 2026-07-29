package protocol

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
	EventFriendRequestCreated = "friend_request_created" // 好友申请提醒
	EventReadMessageAck       = "msg_read_ack"           // 前端 -> 后端
	EventReadMessageCommitted = "msg_read_committed"     // 后端内部 outbox/MQ
	EventReadMessageNotify    = "msg_read_notify"        // 发送给前端，表示已经有用户读取消息
	EventTypeMsgAck           = "msg_ack"                // 发送给前端，表示已经接收到消息
	EventTypeSendMessage      = "msg"                    // 后端发送给前端，表示新的消息
	EventRoomMessageNotice    = "room_msg_notice"        // 大群轻量新消息提醒，客户端按 seq 拉取详情
	EventRoomMemberChanged    = "room_member_changed"
	EventMessageBatch         = "msg_batch" // 服务端一次性发送多个数据
)

type MessageReadAckEvent struct {
	UserId         string   `json:"userId"`
	ConversationId string   `json:"conversationId"`
	LastReadSeq    int64    `json:"lastReadSeq"`
	ConvType       ConvType `json:"convType"`
	SenderId       string   `json:"senderId"`
	Avatar         string   `json:"avatar,omitempty"`
}

type MessageReadCommittedEvent struct {
	ReaderId       string   `json:"readerId"`
	ConversationId string   `json:"conversationId"`
	OldReadSeq     int64    `json:"oldReadSeq"`
	LastReadSeq    int64    `json:"lastReadSeq"`
	ConvType       ConvType `json:"convType"`
	NotifyUserIds  []string `json:"notifyUserIds"`
	Avatar         string   `json:"avatar,omitempty"`
}

type FriendRequestCreatedEvent struct {
	RequestId  string `json:"requestId"`
	FromUserId string `json:"fromUserId"`
	ToUserId   string `json:"toUserId"`
	Message    string `json:"message,omitempty"`
	ApplyTime  int64  `json:"applyTime"`
}

type MessageAckEvent struct {
	ClientMsgId    string    `json:"clientMsgId"`
	MessageId      string    `json:"messageId"`
	ConversationId string    `json:"conversationId,omitempty"`
	Seq            int64     `json:"seq,omitempty"`
	Status         AckStatus `json:"status"`
	Extra          string    `json:"extra"`
	SendTime       int64     `json:"sendTime"`
}

type MessageNotifyEvent struct {
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
	Seq            int64  `json:"seq"`
}

type RoomMemberChangedEvent struct {
	RoomID    string `json:"roomId"`
	UserID    string `json:"userId"`
	Status    int    `json:"status"`
	Role      int    `json:"role"`
	MuteUntil *int64 `json:"muteUntil,omitempty"`
	Version   int64  `json:"version"`
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
