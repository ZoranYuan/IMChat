package room

import (
	roomentity "IM_backend/internal/domain/room/entity"

	"gorm.io/gorm"
)

type RoomUserRepository interface {
	JoinRoom(*roomentity.RoomUser) (*roomentity.RoomUser, error)
	WithTx(*gorm.DB) RoomUserRepository
	GetRelationByIDs(string, string) (*roomentity.RoomUser, error)
	ListActiveUserIDs(roomId string) ([]string, error)
	LeaveRoom(*roomentity.RoomUser, []int) error
}
