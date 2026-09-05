package message

import (
	"context"
	"time"
)

type MessageCache interface {
	SetDedupEntry(ctx context.Context, senderID, clientMsgID string, entry DedupEntry, ttl time.Duration) (bool, error)
	GetDedupEntry(ctx context.Context, senderID, clientMsgID string) (DedupEntry, error)
}

type DedupEntry struct {
	MessageID      string `json:"messageId"`
	RequestHash    string `json:"requestHash"`
	ConversationID string `json:"conversationId"`
	Seq            int64  `json:"seq"`
	AttachmentID   string `json:"attachmentId,omitempty"`
	SendTime       int64  `json:"sendTime"`
	SenderUsername string `json:"senderUsername,omitempty"`
}
