package room_user_repository

import (
	room_entity "IM_backend/internal/domain/room/entity"
	room_repository_interface "IM_backend/internal/domain/room/repository"
	"IM_backend/internal/infrastructure/database/mysql/model"
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

func (rur *RoomUserRepository) WithTx(tx *gorm.DB) room_repository_interface.RoomUserRepositoryInterface {
	return &RoomUserRepository{
		db: tx,
	}
}

func (rur *RoomUserRepository) GetRelationByIds(userId, roomId string) (*room_entity.RoomUser, error) {
	var m model.RoomUser
	if err := rur.db.Where("user_id = ? AND room_id = ?", userId, roomId).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, room_entity.ErrRecordNotFound
		}
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
		return nil, room_entity.ErrDuplicateCreate
	}

	return ToDomain(model), nil
}

func (rur *RoomUserRepository) JoinRoom(domain *room_entity.RoomUser) error {
	var m = ToModel(domain)
	sql := `
		INSERT INTO room_user(
			room_id,
			user_id,
			role,
			status,
			mute_util,
			join_time,
			leave_time,
			version
		)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			status = IF(version = ?, VALUES(status), status),
			join_time = IF(version = ?, VALUES(join_time), join_time),
			leave_time = IF(version = ?, VALUES(leave_time), leave_time),
			mute_util = IF(version = ?, VALUES(mute_util), mute_util),
    		version = IF(version = ?, version + 1, version)
	`

	res := rur.db.Exec(sql,
		m.RoomId,
		m.UserId,
		m.Role,
		m.Status,
		m.MuteUtil,
		m.JoinTime,
		m.LeaveTime,
		m.Version,

		m.Version,
		m.Version,
		m.Version,
		m.Version,
		m.Version,
	)

	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return room_entity.ErrVersionConflict
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
