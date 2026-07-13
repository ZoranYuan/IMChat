package roomuser

import (
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
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

func (rur *RoomUserRepository) WithTx(tx *gorm.DB) roomrepo.RoomUserRepository {
	return &RoomUserRepository{
		db: tx,
	}
}

func (rur *RoomUserRepository) GetRelationByIDs(userId, roomId string) (*roomentity.RoomUser, error) {
	var m model.RoomUser
	if err := rur.db.Where("user_id = ? AND room_id = ?", userId, roomId).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, roomentity.ErrMemberNotFound
		}

		return nil, err
	}

	return ToDomain(m), nil
}

func (rur *RoomUserRepository) ListActiveUserIDs(roomId string) ([]string, error) {
	var userIds []string
	err := rur.db.Model(&model.RoomUser{}).
		Where("room_id = ? AND status IN ?", roomId, []int{int(roomvo.Activate), int(roomvo.BeMuted)}).
		Pluck("user_id", &userIds).Error
	return userIds, err
}

func (rur *RoomUserRepository) JoinRoom(domain *roomentity.RoomUser) (*roomentity.RoomUser, error) {
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
		return nil, roomentity.ErrDuplicateCreation
	}

	return ToDomain(model), nil
}

func (rur *RoomUserRepository) LeaveRoom(domain *roomentity.RoomUser, status []int) error {
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
		return roomentity.ErrVersionConflict
	}

	return nil
}
