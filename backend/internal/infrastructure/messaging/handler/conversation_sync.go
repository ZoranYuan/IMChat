package mq_handler

import (
	conversationapp "IM_backend/internal/application/conversation"
	"context"
)

type ConversationSyncService interface {
	SyncLatestSequences(ctx context.Context, items []conversationapp.SyncSeq) error
}
