package friend_request_repository

import (
	friend_request_entity "IM_backend/internal/domain/frient_request/entity"
	"IM_backend/internal/infrastructure/database/mysql/model"
	"errors"

	"gorm.io/gorm"
)

type friendRequestRepo struct {
	db *gorm.DB
}

func NewFriendRequestRepository(db *gorm.DB) *friendRequestRepo {
	return &friendRequestRepo{
		db: db,
	}
}

func (fr *friendRequestRepo) FindByUsers(userId, toUserId string) (*friend_request_entity.FriendRequest, error) {
	var friendRequestModel model.FriendRequest
	err := fr.db.
		Where("from_user_id = ? AND to_user_id = ?", userId, toUserId).
		First(&friendRequestModel).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return toDomain(&friendRequestModel), nil
}

func (fr *friendRequestRepo) Create(domain *friend_request_entity.FriendRequest) (*friend_request_entity.FriendRequest, error) {
	m := toModel(domain)

	if err := fr.db.Create(&m).Error; err != nil {
		return nil, err
	}

	return toDomain(m), nil
}

func (fr *friendRequestRepo) Update(domain *friend_request_entity.FriendRequest) error {
	m := toModel(domain)

	updates := map[string]interface{}{
		"status":  m.Status,
		"message": m.Message,
	}

	return fr.db.Model(&model.FriendRequest{}).
		Where("from_user_id = ? AND to_user_id = ?", m.FromUserId, m.ToUserId).
		Updates(updates).Error
}
