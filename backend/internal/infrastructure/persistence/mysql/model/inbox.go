package model

import "time"

type InboxRecord struct {
	EventID     string     `gorm:"primaryKey;size:64"`
	EventType   string     `gorm:"size:64;not null"`
	Status      string     `gorm:"size:16;not null;default:'completed';index:idx_inbox_status_locked,priority:1"`
	LockedAt    *time.Time `gorm:"index:idx_inbox_status_locked,priority:2"`
	LockToken   string     `gorm:"size:64;not null;default:'';index:idx_inbox_status_locked,priority:3"`
	RetryCount  int        `gorm:"not null;default:0"`
	LastError   string     `gorm:"type:text"`
	ProcessedAt time.Time  `gorm:"not null"`
}

func (InboxRecord) TableName() string {
	return "inboxes"
}
