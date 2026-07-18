package outbox

import "time"

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusSent       = "sent"
	StatusDead       = "dead"
)

type Entry struct {
	ID          string
	EventType   string
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
