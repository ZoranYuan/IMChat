package message

import (
	"context"
	"time"
)

type MessageCache interface {
	SetDedupEntry(ctx context.Context, sendID, clientMsgID, messageID, requestHash string, ttl time.Duration) (bool, error)
	GetDedupEntry(ctx context.Context, sendID, clientMsgID string) (DedupEntry, error)
}

type DedupEntry struct {
	MessageID   string `json:"messageId"`
	RequestHash string `json:"requestHash"`
}
