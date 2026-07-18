package conversation

import "context"

type SyncItem struct {
	UserId         string
	ConversationId string
	LatestSeq      int64
}

type Synchronizer interface {
	SyncLatestSequences(ctx context.Context, items []SyncItem) error
}
