package room

import (
	roomentity "IM_backend/internal/domain/room/entity"

	"gorm.io/gorm"
)

type RoomUserRepository interface {
	Create(*roomentity.RoomUser) (*roomentity.RoomUser, error)
	WithTx(*gorm.DB) RoomUserRepository
	GetRelationByIDs(string, string) (*roomentity.RoomUser, error)
	JoinRoom(*roomentity.RoomUser) error
	Leave(*roomentity.RoomUser, []int) error
}
