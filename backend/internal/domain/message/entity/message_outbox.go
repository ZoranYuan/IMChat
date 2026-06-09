package entity

import "time"

const (
	MessageOutboxStatusPending    = "pending"
	MessageOutboxStatusProcessing = "processing"
	MessageOutboxStatusSent       = "sent"
)

type MessageOutbox struct {
	ID          string
	EventType   string
	Topic       string
	MessageKey  string
	Payload     []byte
	Status      string
	RetryCount  int
	NextRetryAt time.Time
	LockedAt    *time.Time
	LastError   string
	SentAt      *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
