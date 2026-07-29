package message

import (
	"context"
	"time"
)

type MessageCache interface {
	SetDedupEntry(ctx context.Context, sendID, clientMsgID, messageID string, ttl time.Duration) (bool, error)
	GetDedupEntry(ctx context.Context, sendID, clientMsgID string) (string, error)
}
