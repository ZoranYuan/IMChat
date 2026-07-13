package conversation

import (
	"context"
	"time"
)

type ConversationCache interface {
	IsMemberWithVersion(ctx context.Context, convId, userId string) (bool, int64, error)
	SetMembers(ctx context.Context, convId string, userIds []string, version int64) error
	DeleteConversation(ctx context.Context, convID string) error
	IncrConvLatestSeq(ctx context.Context, convId string) (int64, error)
	GetConvLatestSeq(ctx context.Context, convId string) (int64, error)
	SetConvSeq(ctx context.Context, convId string, seq int64) error
	AddMember(ctx context.Context, convId, userId string, version int64) error
	RemoveMember(ctx context.Context, convId, userId string, version int64) error
	GetMembersWithVersion(
		ctx context.Context,
		convId string,
	) ([]string, int64, error)
	// DedupEntry atomically stores clientMsgId → messageId mapping.
	// Returns false if the key already exists (duplicate request).
	SetDedupEntry(ctx context.Context, clientMsgId, messageId string, ttl time.Duration) (bool, error)
	GetDedupEntry(ctx context.Context, clientMsgId string) (string, error)
}
