package model

import "time"

type InboxRecord struct {
	EventID     string    `gorm:"primaryKey;size:64"`
	EventType   string    `gorm:"size:64;not null"`
	ProcessedAt time.Time `gorm:"not null"`
}

func (InboxRecord) TableName() string {
	return "inboxes"
}
