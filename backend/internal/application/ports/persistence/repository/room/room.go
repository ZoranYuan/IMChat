package room

import (
	roomentity "IM_backend/internal/domain/room/entity"

	"gorm.io/gorm"
)

type RoomRepository interface {
	Create(domain *roomentity.Room) error
	WithTx(tx *gorm.DB) RoomRepository
	UpdateRoomVersion(roomId string) (int64, error)
	FindActiveRoom(roomId string, status int) (*roomentity.Room, error)
}
