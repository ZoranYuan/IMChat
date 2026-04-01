package room_repository

import (
	room_entity "IM_backend/internal/domain/room/entity"
	room_repository_interface "IM_backend/internal/domain/room/repository"
	"IM_backend/internal/infrastructure/database/mysql/model"
	"errors"

	"gorm.io/gorm"
)

type RoomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) room_repository_interface.RoomRepositoryInterface {
	return &RoomRepository{
		db: db,
	}
}

func (rr *RoomRepository) Create(domain *room_entity.Room) (*room_entity.Room, error) {
	model := toModel(domain)

	if err := rr.db.Create(&model).Error; err != nil {
		return nil, err
	}

	return toDomain(model), nil
}

func (rr *RoomRepository) FindRoomByRoomId(roomId string) (*room_entity.Room, error) {
	var m model.Room

	if err := rr.db.Where("room_id = ?", roomId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, room_entity.ErrRoomNotFound
		}

		return nil, err
	}

	return toDomain(m), nil
}

func (rr *RoomRepository) WithTx(tx *gorm.DB) room_repository_interface.RoomRepositoryInterface {
	return &RoomRepository{
		db: tx,
	}
}
