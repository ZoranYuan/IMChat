package message_cache_interface

import "context"

type MessageCacheInterface interface {
	IncrMessageLatestSeq(ctx context.Context, convId string) (int64, error)
	GetMessageLatestSeq(ctx context.Context, convId string) (int64, error)
	SetMessageSeq(ctx context.Context, convId string, seq int64) error
}
