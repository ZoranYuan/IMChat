package room

import (
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	roomentity "IM_backend/internal/domain/room/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"errors"

	"gorm.io/gorm"
)

type RoomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) *RoomRepository {
	return &RoomRepository{
		db: db,
	}
}

func (rr *RoomRepository) Create(domain *roomentity.Room) error {
	model := toModel(domain)

	return rr.db.Create(&model).Error
}

func (rr *RoomRepository) UpdateRoomVersion(roomId string) (int64, error) {
	res := rr.db.
		Model(&model.Room{}).
		Where("room_id = ?", roomId).
		Update("version", gorm.Expr("version + 1"))

	if res.Error != nil {
		return 0, res.Error
	}

	if res.RowsAffected == 0 {
		return 0, roomentity.ErrRoomNotFound
	}

	// Step 2: 查回 version（关键）
	var version int64
	err := rr.db.
		Model(&model.Room{}).
		Select("version").
		Where("room_id = ?", roomId).
		Scan(&version).Error

	if err != nil {
		return 0, err
	}

	return version, nil
}

func (rr *RoomRepository) FindActiveRoom(roomId string, status int) (*roomentity.Room, error) {
	var m model.Room

	if err := rr.db.Where("room_id = ? AND status = ?", roomId, status).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, roomentity.ErrRoomNotFound
		}

		return nil, err
	}

	return toDomain(m), nil
}

func (rr *RoomRepository) WithTx(tx *gorm.DB) roomrepo.RoomRepository {
	return &RoomRepository{
		db: tx,
	}
}
