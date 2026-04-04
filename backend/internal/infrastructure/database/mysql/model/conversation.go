package model

import "time"

type Conversation struct {
	ConversationId string `gorm:"size:64;not null;primaryKey"`

	Type int8 `gorm:"tinyInt;not null;comment: (单聊 1/群聊 2)"`

	UserId1 string `gorm:"size:32;not null;index"`
	UserId2 string `gorm:"size:32;not null;index"`

	RoomId string `gorm:"size:32;not null;index"`

	// 当前会话的最大 seq
	LastSeq int64 `gorm:"not null;default:0"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
