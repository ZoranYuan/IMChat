package room_cache_interface

import "context"

type RoomCache interface {
	GetRoomIDByCode(ctx context.Context, code string) (string, error)
	GetInviteCode(ctx context.Context, roomId string) (string, error)
	UpdateInviteCode(ctx context.Context, roomId string, ttl int) (string, error)
}
