package room_cache_interface

import "context"

type RoomCacheInterface interface {
	UpdateInviteCode(ctx context.Context, roomId string, ttl int) (string, error)
}
