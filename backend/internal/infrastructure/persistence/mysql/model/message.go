package model

type Message struct {
	MessageId      string  `gorm:"size:32;primaryKey" json:"messageId"`
	ConversationId string  `gorm:"size:64;not null;uniqueIndex:uk_message_conv_seq,priority:1" json:"conversationId"`
	SenderId       string  `gorm:"size:32;not null;uniqueIndex:uq_sender_client_msg,priority:1" json:"senderId"`
	ClientMsgId    *string `gorm:"size:64;uniqueIndex:uq_sender_client_msg,priority:2" json:"clientMsgId,omitempty"`
	RequestHash    string  `gorm:"size:64;default:''" json:"-"`

	Seq      int64  `gorm:"not null;uniqueIndex:uk_message_conv_seq,priority:2" json:"seq"`
	Type     int8   `gorm:"not null;comment:1=text 2=image 3=video 4=sticker 5=file" json:"type"`
	Content  string `gorm:"type:text" json:"content"`
	Status   int8   `gorm:"default:1;comment:1=normal 2=recall" json:"status"`
	SendTime int64  `gorm:"not null;index:idx_send_time" json:"sendTime"`
}
