package room

import (
	roomentity "IM_backend/internal/domain/room/entity"
)

type RoomUserRepository interface {
	JoinRoom(*roomentity.RoomUser) (*roomentity.RoomUser, error)
	WithTx(any) RoomUserRepository
	GetRelationByIDs(string, string) (*roomentity.RoomUser, error)
	ListActiveUserIDs(roomId string) ([]string, error)
	LeaveRoom(*roomentity.RoomUser, []int) error
}
