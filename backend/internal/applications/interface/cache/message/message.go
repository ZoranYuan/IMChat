package message_cache_interface

import "context"

type MessageCacheInterface interface {
	GetConvLatestSeq(ctx context.Context, convId string) (int64, error)
	SetConvSeq(ctx context.Context, convId string, seq int64) error
	IncrConvSeq(ctx context.Context, convId string) (int64, error)
}
