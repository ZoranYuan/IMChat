package outbox

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, outbox *Entry) error
	ClaimPending(
		ctx context.Context,
		now time.Time,
		staleBefore time.Time,
		limit int,
	) ([]*Entry, error)
	MarkSent(ctx context.Context, id, lockToken string, sentAt time.Time) error
	MarkRetry(ctx context.Context, id, lockToken string, nextRetryAt time.Time, lastError string) error
	MarkDead(ctx context.Context, id, lockToken string, lastError string) error
	WithTx(tx any) Repository
}
