package model

import "time"

type UserConversation struct {
	UserId         string `gorm:"size:32;primaryKey"`
	ConversationId string `gorm:"size:64;primaryKey"`

	LatestReadSeq int64 `gorm:"not null;default:0"`

	IsMuted bool `gorm:"not null;defalt:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
