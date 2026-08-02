package inbox

import (
	"context"
	"time"
)

const (
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusDead       = "dead"
)

type InboxRepository interface {
	TryClaim(ctx context.Context, eventID, eventType string, now, staleBefore time.Time) (bool, error)
	MarkCompleted(ctx context.Context, eventID string, processedAt time.Time) error
	MarkDead(ctx context.Context, eventID string, lastError string, processedAt time.Time) error
	WithTx(tx any) InboxRepository
}
