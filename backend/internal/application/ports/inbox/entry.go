package inbox

import (
	"context"
	"errors"
	"time"
)

const (
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusDead       = "dead"
)

var ErrLeaseLost = errors.New("inbox lease lost")

type InboxRepository interface {
	TryClaim(ctx context.Context, eventID, eventType string, now, staleBefore time.Time) (bool, string, error)
	MarkCompleted(ctx context.Context, eventID, lockToken string, processedAt time.Time) error
	MarkDead(ctx context.Context, eventID, lockToken string, lastError string, processedAt time.Time) error
	WithTx(tx any) InboxRepository
}
