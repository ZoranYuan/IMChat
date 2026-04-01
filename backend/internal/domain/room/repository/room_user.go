package room_repository_interface

import (
	room_entity "IM_backend/internal/domain/room/entity"

	"gorm.io/gorm"
)

type RoomUserRepositoryInterface interface {
	Create(domain *room_entity.RoomUser) (*room_entity.RoomUser, error)
	WithTx(tx *gorm.DB) RoomUserRepositoryInterface
}
