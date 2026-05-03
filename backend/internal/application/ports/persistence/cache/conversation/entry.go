package conversation

import "context"

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
}
