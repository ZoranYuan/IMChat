package conversation

import "context"

type ConversationCache interface {
	IncrConvLatestSeq(ctx context.Context, convId string) (int64, error)
	GetConvLatestSeq(ctx context.Context, convId string) (int64, error)
	SetConvSeq(ctx context.Context, convId string, seq int64) error
}
