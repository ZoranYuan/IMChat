package mq_interface

import "context"

type RoomResolver interface {
	GetRoomMembers(ctx context.Context, roomId string) ([]string, error)
}
