package room_repository_interface

import (
	room_entity "IM_backend/internal/domain/room/entity"

	"gorm.io/gorm"
)

type RoomRepository interface {
	Create(domain *room_entity.Room) error
	WithTx(tx *gorm.DB) RoomRepository
	UpdateRoomVersion(roomId string) (int64, error)
	FindActiveRoom(roomId string, status int) (*room_entity.Room, error)
}
