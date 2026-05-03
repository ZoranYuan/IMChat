package friendrequest

import (
	friendrequestrepo "IM_backend/internal/application/ports/persistence/repository/friend_request"
	friendrequestentity "IM_backend/internal/domain/friend_request/entity"
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

func (r *FriendRequestRepo) WithTx(tx *gorm.DB) friendrequestrepo.FriendRequestRepository {
	return &FriendRequestRepo{db: tx}
}

func (fr *FriendRequestRepo) FindLatestRequest(userId, toUserId string) (*friendrequestentity.FriendRequest, error) {
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

func (fr *FriendRequestRepo) Create(domain *friendrequestentity.FriendRequest) (*friendrequestentity.FriendRequest, error) {
	m := toModel(domain)

	if err := fr.db.Create(&m).Error; err != nil {
		return nil, err
	}

	return toDomain(m), nil
}

func (fr *FriendRequestRepo) ReRequest(domain *friendrequestentity.FriendRequest) error {
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
		return friendrequestentity.ErrInvalidStatusTransition
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
		return friendrequestentity.ErrInvalidStatusTransition
	}

	return nil
}

func (fr *FriendRequestRepo) ListByUserID(userId string) ([]*friendrequestentity.FriendRequest, error) {
	var m []model.FriendRequest
	if err := fr.db.Where("to_user_id = ?", userId).
		Order("updated_at DESC").
		Find(&m).Error; err != nil {
		return nil, err
	}

	domainList := make([]*friendrequestentity.FriendRequest, 0, len(m))

	for _, m := range m {
		domainList = append(domainList, toDomain(m))
	}

	return domainList, nil
}

func (fr *FriendRequestRepo) FindByRequestID(requestId string) (*friendrequestentity.FriendRequest, error) {
	var m model.FriendRequest

	if err := fr.db.Where("request_id = ?", requestId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, friendrequestentity.ErrFriendRequestNotFound
		}
		return nil, err
	}

	return toDomain(m), nil
}
