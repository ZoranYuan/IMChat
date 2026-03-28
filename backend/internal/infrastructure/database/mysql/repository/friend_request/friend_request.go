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

func NewFriendRequestRepository(db *gorm.DB) friendRequestRepo {
	return friendRequestRepo{
		db: db,
	}
}

func (fr friendRequestRepo) FindByUsersByIds(userId, toUserId string) (*friend_request_entity.FriendRequest, error) {
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

	return toDomain(friendRequestModel), nil
}

func (fr friendRequestRepo) Create(domain *friend_request_entity.FriendRequest) (*friend_request_entity.FriendRequest, error) {
	m := toModel(domain)

	if err := fr.db.Create(&m).Error; err != nil {
		return nil, err
	}

	return toDomain(m), nil
}

func (fr friendRequestRepo) ReRequest(domain *friend_request_entity.FriendRequest, expectStatus []int) error {
	m := toModel(domain)

	result := fr.db.Model(&model.FriendRequest{}).
		Where(
			"request_id = ? AND status IN ?",
			m.RequestId,
			expectStatus,
		).
		Updates(map[string]interface{}{
			"status":  m.Status,
			"message": m.Message,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("invalid state transition")
	}

	return nil
}

func (fr friendRequestRepo) OperateRequest(requestId string, expectStatus, newStatus int) error {
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
		return errors.New("invalid state transition")
	}

	return nil
}

func (fr friendRequestRepo) ListByUserId(userId string) ([]*friend_request_entity.FriendRequest, error) {
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

func (fr friendRequestRepo) FindByRequestId(requestId string) (*friend_request_entity.FriendRequest, error) {
	var m model.FriendRequest

	if err := fr.db.Where("request_id = ?", requestId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("unknown params") // 或 domain not found
		}
		return nil, err
	}

	return toDomain(m), nil
}
