package message

import (
	"context"
	"time"

	messageentity "IM_backend/internal/domain/message/entity"
)

type MessageOutboxRepository interface {
	Create(ctx context.Context, outbox *messageentity.MessageOutbox) error
	ClaimPending(
		ctx context.Context,
		now time.Time,
		staleBefore time.Time,
		limit int,
	) ([]*messageentity.MessageOutbox, error)
	MarkSent(ctx context.Context, id string, sentAt time.Time) error
	MarkRetry(ctx context.Context, id string, nextRetryAt time.Time, lastError string) error
	WithTx(tx any) MessageOutboxRepository
}
