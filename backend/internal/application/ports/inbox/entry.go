package inbox

import "context"

type InboxRepository interface {
	TryInsert(ctx context.Context, eventID, eventType string) (bool, error)
	WithTx(tx any) InboxRepository
}
