package room_repository_interface

import (
	room_entity "IM_backend/internal/domain/room/entity"

	"gorm.io/gorm"
)

type RoomRepositoryInterface interface {
	Create(domain *room_entity.Room) (*room_entity.Room, error)
	WithTx(tx *gorm.DB) RoomRepositoryInterface
	FindActiveRoom(roomId string, status int) (*room_entity.Room, error)
}
