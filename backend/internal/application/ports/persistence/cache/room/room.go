package room

import (
	roomvo "IM_backend/internal/domain/room/value_object"
	"context"
	"time"
)

type MemberState struct {
	Status    roomvo.RoomUserStatus
	Role      roomvo.Role
	MuteUntil *int64
	Version   int64
}

type RoomCache interface {
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
