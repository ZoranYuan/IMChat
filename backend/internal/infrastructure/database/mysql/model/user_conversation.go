package model

import "time"

type UserConversation struct {
	UserId         string    `json:"userId" gorm:"size:32;primaryKey"`
	ConversationId string    `json:"conversationId" gorm:"size:64;primaryKey"`
	LastMessageId  string    `json:"lastMessageId" gorm:"size:32;uniqueIndex"`
	LastReadSeq    int64     `json:"lastReadSeq" gorm:"not null;default:0"`
	LatestSyncSeq  int64     `json:"latestSyncReq" gorm:"column:latest_sync_seq"`
	IsMuted        bool      `json:"isMuted" gorm:"not null;default:false"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
