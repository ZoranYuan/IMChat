package friend_request_repository

import (
	friend_request_repository_interface "IM_backend/internal/application/ports/repository/friend_request"
	friend_request_entity "IM_backend/internal/domain/friend_request/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"errors"

	"gorm.io/gorm"
)

type FriendRequestRepo struct {
	db *gorm.DB
}

func NewFriendRequestRepository(db *gorm.DB) *FriendRequestRepo {
	return &FriendRequestRepo{
		db: db,
	}
}

func (r *FriendRequestRepo) WithTx(tx *gorm.DB) friend_request_repository_interface.FriendRequestRepository {
	return &FriendRequestRepo{db: tx}
}

func (fr *FriendRequestRepo) FindLatestRequest(userId, toUserId string) (*friend_request_entity.FriendRequest, error) {
	var friendRequestModel model.FriendRequest
	err := fr.db.
		Where("from_user_id = ? AND to_user_id = ?", userId, toUserId).
		Order("created_at DESC").
		First(&friendRequestModel).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return toDomain(friendRequestModel), nil
}

func (fr *FriendRequestRepo) Create(domain *friend_request_entity.FriendRequest) (*friend_request_entity.FriendRequest, error) {
	m := toModel(domain)

	if err := fr.db.Create(&m).Error; err != nil {
		return nil, err
	}

	return toDomain(m), nil
}

func (fr *FriendRequestRepo) ReRequest(domain *friend_request_entity.FriendRequest) error {
	m := toModel(domain)

	result := fr.db.Model(&model.FriendRequest{}).
		Where(
			"request_id = ? AND status = ?",
			m.RequestId,
			m.Status,
		).
		Updates(map[string]interface{}{
			"message": m.Message,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return friend_request_entity.ErrInvalidStatusTransition
	}

	return nil
}

func (fr *FriendRequestRepo) OperateRequest(requestId string, expectStatus, newStatus int) error {
	result := fr.db.Model(&model.FriendRequest{}).
		Where(
			"request_id = ? AND status = ?",
			requestId,
			expectStatus,
		).
		Updates(map[string]interface{}{
			"status": newStatus,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return friend_request_entity.ErrInvalidStatusTransition
	}

	return nil
}

func (fr *FriendRequestRepo) ListByUserID(userId string) ([]*friend_request_entity.FriendRequest, error) {
	var m []model.FriendRequest
	if err := fr.db.Where("to_user_id = ?", userId).
		Order("updated_at DESC").
		Find(&m).Error; err != nil {
		return nil, err
	}

	domainList := make([]*friend_request_entity.FriendRequest, 0, len(m))

	for _, m := range m {
		domainList = append(domainList, toDomain(m))
	}

	return domainList, nil
}

func (fr *FriendRequestRepo) FindByRequestID(requestId string) (*friend_request_entity.FriendRequest, error) {
	var m model.FriendRequest

	if err := fr.db.Where("request_id = ?", requestId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, friend_request_entity.ErrFriendRequestNotFound
		}
		return nil, err
	}

	return toDomain(m), nil
}
