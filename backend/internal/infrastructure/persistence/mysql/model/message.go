package model

type Message struct {
	MessageId      string  `gorm:"size:32;primaryKey" json:"messageId"`
	ConversationId string  `gorm:"size:64;not null;index:idx_conv_seq,priority:1;uniqueIndex:uk_message_conv_seq,priority:1" json:"conversationId"`
	SendId         string  `gorm:"size:32;not null;index:idx_sender;uniqueIndex:uq_sender_client_msg,priority:1" json:"sendId"`
	ClientMsgId    *string `gorm:"size:64;uniqueIndex:uq_sender_client_msg,priority:2" json:"clientMsgId,omitempty"`
	RequestHash    string  `gorm:"size:64;default:'';index" json:"-"`

	Seq     int64  `gorm:"not null;index:idx_conv_seq,priority:2,sort:desc;uniqueIndex:uk_message_conv_seq,priority:2" json:"seq"`
	Type    int8   `gorm:"not null;comment:1=text 2=image 3=video 4=sticker 5=file" json:"type"`
	Content string `gorm:"type:text" json:"content"`
	// 弹幕/视频时间戳关联（非媒体字段）
	VideoId   string `gorm:"size:64;index:idx_video_time,priority:1;comment:video file id" json:"videoId,omitempty"`
	VideoTime *int64 `gorm:"index:idx_video_time,priority:2;comment:video progress(ms)" json:"videoTime,omitempty"`
	Status    int8   `gorm:"default:1;comment:1=normal 2=recall" json:"status"`
	SendTime  int64  `gorm:"not null;index:idx_send_time" json:"sendTime"`
}
