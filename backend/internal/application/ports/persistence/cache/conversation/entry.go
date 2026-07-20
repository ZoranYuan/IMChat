package conversation

import (
	"context"
	"errors"
)

var ErrConversationNotFound = errors.New("会话不存在")

type ConversationCache interface {
	IncrConvLatestSeq(
		ctx context.Context,
		conversationID string,
	) (int64, error)

	RecoverConvLatestSeq(
		ctx context.Context,
		conversationID string,
		dbLatestSeq int64,
	) error

	GetConvLatestSeq(
		ctx context.Context,
		conversationID string,
	) (int64, error)

	MarkConversationNotFound(ctx context.Context, conversationID string) error
}
