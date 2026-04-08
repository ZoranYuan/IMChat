package room_cache_interface

import "context"

type RoomCacheInterface interface {
	GetRoomIdByCode(ctx context.Context, code string) (string, error)
	GetInviteCode(ctx context.Context, roomId string) (string, error)
	UpdateInviteCode(ctx context.Context, roomId string, ttl int) (string, error)
	JoinRoom(ctx context.Context, roomId, userId string) error
	LeaveRoom(ctx context.Context, roomId, userId string) error
}
