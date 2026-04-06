package model

import "time"

type Message struct {
	MessageId      string `gorm:"size:32;primaryKey"`
	ConversationId string `gorm:"size:64;not null;index:idx_conv_seq,priority:1"`

	SendId string `gorm:"size:32;not null;index"`

	// 当前消息的自增序列值
	Seq int64 `gorm:"not null;index:idx_conv_seq,priority:2"`
	// 消息状态
	Type    int8   `gorm:"not null;comment:1=文本 2=图片 3=视频"`
	Content string `gorm:"type:text"`
	// 消息状态
	VideoTime int64     `gorm:"not null;comment:消息对应的视频时间"`
	Status    int8      `gorm:"default:0;comment:1=正常 2=撤回"`
	CreatedAt time.Time `gorm:"index"`
}
