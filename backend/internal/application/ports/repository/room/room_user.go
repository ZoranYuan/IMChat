package room_repository_interface

import (
	room_entity "IM_backend/internal/domain/room/entity"

	"gorm.io/gorm"
)

type RoomUserRepository interface {
	Create(*room_entity.RoomUser) (*room_entity.RoomUser, error)
	WithTx(*gorm.DB) RoomUserRepository
	GetRelationByIDs(string, string) (*room_entity.RoomUser, error)
	JoinRoom(*room_entity.RoomUser) error
	Leave(*room_entity.RoomUser, []int) error
}
