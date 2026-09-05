package room

import (
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/shared/protocol"
	"context"
	"time"
)

type MemberState struct {
	Status    roomvo.RoomUserStatus
	Role      roomvo.Role
	MuteUntil *int64
	Version   int64
}

type RecentMessageRange struct {
	Events  []protocol.MessageEvent
	Covered bool // 缓存是否完整覆盖 afterSeq 到 latestSeq 的同步区间
}

const (
	RoomActivityNormal = iota
	RoomActivityWarn
	RoomActivityActive
)

type RoomCache interface {
	RecordActivity(ctx context.Context, roomId string) error
	ActivateLevel(ctx context.Context, roomId string) (int, error)
	ListMessageAfterSeq(ctx context.Context, roomId string, afterSeq, latestSeq int64) (RecentMessageRange, error)
	AppendRecentMessageSeq(ctx context.Context, roomId string, event protocol.MessageEvent) error
	WarmRecentMessageEvents(ctx context.Context, roomId string, events []protocol.MessageEvent) error
	GetRoomIDByCode(ctx context.Context, code string) (string, error)
	GetInviteCode(ctx context.Context, roomId string) (string, error)
	UpdateInviteCode(ctx context.Context, roomId string, ttl time.Duration) (string, error)
	DeleteInviteCode(ctx context.Context, roomId string) error
}

type RoomMemberCache interface {
	GetMember(ctx context.Context, roomID, userID string) (*MemberState, bool, error)
	SetMember(ctx context.Context, roomID, userID string, state *MemberState) error
	SetMemberIfVersionGreater(ctx context.Context, roomID, userID string, state *MemberState) (bool, error)
	SetMemberNotFound(ctx context.Context, roomID, userID string) error
	DeleteMember(ctx context.Context, roomID, userID string) error
	GetMemberIDs(ctx context.Context, roomID string) ([]string, bool, error)
	SetMemberIDs(ctx context.Context, roomID string, userIDs []string) error
	DeleteMemberIDs(ctx context.Context, roomID string) error
}
