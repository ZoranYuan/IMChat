package room

import (
	roomentity "IM_backend/internal/domain/room/entity"
)

type RoomRepository interface {
	Create(domain *roomentity.Room) error
	WithTx(tx any) RoomRepository
	FindActiveRoom(roomId string, status int) (*roomentity.Room, error)
}
