package room_user_repository

import (
	room_repository_interface "IM_backend/internal/application/ports/repository/room"
	room_entity "IM_backend/internal/domain/room/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RoomUserRepository struct {
	db *gorm.DB
}

func NewRoomUserRepository(db *gorm.DB) *RoomUserRepository {
	return &RoomUserRepository{
		db: db,
	}
}

func (rur *RoomUserRepository) WithTx(tx *gorm.DB) room_repository_interface.RoomUserRepository {
	return &RoomUserRepository{
		db: tx,
	}
}

func (rur *RoomUserRepository) GetRelationByIDs(userId, roomId string) (*room_entity.RoomUser, error) {
	var m model.RoomUser
	if err := rur.db.Where("user_id = ? AND room_id = ?", userId, roomId).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, room_entity.ErrMemberNotFound
		}

		return nil, err
	}

	return ToDomain(m), nil
}

func (rur *RoomUserRepository) Create(domain *room_entity.RoomUser) (*room_entity.RoomUser, error) {
	model := ToModel(domain)

	res := rur.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{
			Name: "room_id",
		}, {
			Name: "user_id",
		},
		},
		DoNothing: true,
	}).Create(&model)

	if res.Error != nil {
		return nil, res.Error
	}

	if res.RowsAffected == 0 {
		return nil, room_entity.ErrDuplicateCreation
	}

	return ToDomain(model), nil
}

func (rur *RoomUserRepository) JoinRoom(domain *room_entity.RoomUser) error {
	var m = ToModel(domain)

	result := rur.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "room_id"},
			{Name: "user_id"},
		},
		DoNothing: true,
	}).Create(&m)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return room_entity.ErrDuplicateJoin
	}

	return nil
}

func (rur *RoomUserRepository) Leave(domain *room_entity.RoomUser, status []int) error {
	var m = ToModel(domain)

	updates := map[string]interface{}{
		"status":     m.Status,
		"leave_time": m.LeaveTime,
		"version":    gorm.Expr("version + 1"),
	}
	res := rur.db.Model(&model.RoomUser{}).
		Where("room_id = ? AND user_id = ?", domain.RoomId, domain.UserId).
		Where("status IN ?", status).
		Where("version = ?", m.Version).
		Updates(updates)

	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return room_entity.ErrVersionConflict
	}

	return nil
}
