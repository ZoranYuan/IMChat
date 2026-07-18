package model

import "time"

type OutboxRecord struct {
	ID          string     `json:"id" gorm:"size:32;primaryKey"`
	EventType   string     `json:"eventType" gorm:"size:64;index"`
	Topic       string     `json:"topic" gorm:"size:64;index"`
	MessageKey  string     `json:"messageKey" gorm:"size:128"`
	Payload     []byte     `json:"payload" gorm:"type:longblob"`
	Status      string     `json:"status" gorm:"size:16;index"`
	RetryCount  int        `json:"retryCount" gorm:"not null;default:0"`
	NextRetryAt time.Time  `json:"nextRetryAt" gorm:"not null;index"`
	LockedAt    *time.Time `json:"lockedAt" gorm:"index"`
	LastError   string     `json:"lastError" gorm:"type:text"`
	SentAt      *time.Time `json:"sentAt" gorm:"index"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (OutboxRecord) TableName() string {
	return "message_outboxes"
}
