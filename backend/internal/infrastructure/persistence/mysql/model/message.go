package model

type Message struct {
	MessageId      string `gorm:"size:32;primaryKey" json:"messageId"`
	ConversationId string `gorm:"size:64;not null;index:idx_conv_seq,priority:1" json:"conversationId"`
	SendId         string `gorm:"size:32;not null;index:idx_sender" json:"sendId"`

	// 当前消息的自增序列值
	Seq int64 `gorm:"not null;index:idx_conv_seq,priority:2,sort:desc" json:"seq"`
	// 消息状态
	Type    int8   `gorm:"not null;comment:1=text 2=image 3=video" json:"type"`
	Content string `gorm:"type:text" json:"content"`
	// 消息状态
	VideoTime *int64 `gorm:"comment:video progress(ms)" json:"videoTime,omitempty"`
	Status    int8   `gorm:"default:1;comment:1=normal 2=recall" json:"status"`
	SendTime  int64  `gorm:"not null;index:idx_send_time" json:"sendTime"`
}
