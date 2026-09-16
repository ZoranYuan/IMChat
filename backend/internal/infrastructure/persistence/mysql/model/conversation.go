package model

import "time"

type Conversation struct {
	ConversationId  string    `json:"conversationId" gorm:"size:64;not null;primaryKey"`
	Convtype        int8      `json:"convType" gorm:"tinyInt;not null;comment:(单聊 1/群聊 2)"`
	UserId1         string    `json:"userId1" gorm:"size:32;not null"`
	UserId2         string    `json:"userId2" gorm:"size:32;not null"`
	RoomId          string    `json:"roomId" gorm:"size:32;not null"`
	LatestMessageId string    `json:"latestMessageId" gorm:"size:32;index"`
	LatestSeq       int64     `json:"latestSeq" gorm:"not null;default:0"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
