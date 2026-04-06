package message_cache_interface

import "context"

type MessageCacheInterface interface {
	GetConvLatestSeq(ctx context.Context, convId string) (int64, error)
}
