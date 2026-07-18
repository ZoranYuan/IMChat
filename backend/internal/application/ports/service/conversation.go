package service

import "context"

type ConversationSyncItem struct {
	UserId         string
	ConversationId string
	LatestSeq      int64
}

type ConversationSyncService interface {
	SyncLatestSequences(ctx context.Context, items []ConversationSyncItem) error
}
