package room_user_repository

import (
	room_entity "IM_backend/internal/domain/room/entity"
	room_repository_interface "IM_backend/internal/domain/room/repository"

	"gorm.io/gorm"
)

type RoomUserRepository struct {
	db *gorm.DB
}

func NewRoomUserRepository(db *gorm.DB) room_repository_interface.RoomUserRepositoryInterface {
	return &RoomUserRepository{
		db: db,
	}
}

func (rur *RoomUserRepository) WithTx(tx *gorm.DB) room_repository_interface.RoomUserRepositoryInterface {
	return &RoomUserRepository{
		db: tx,
	}
}

func (rur *RoomUserRepository) Create(domain *room_entity.RoomUser) (*room_entity.RoomUser, error) {
	model := ToModel(domain)

	if err := rur.db.Create(&model).Error; err != nil {
		return nil, err
	}

	return ToDomain(model), nil
}
