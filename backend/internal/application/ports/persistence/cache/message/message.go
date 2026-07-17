package message

import (
	"context"
	"time"
)

type MessageCache interface {
	SetDedupEntry(ctx context.Context, clientMsgID, messageID string, ttl time.Duration) (bool, error)
	GetDedupEntry(ctx context.Context, clientMsgID string) (string, error)
}
