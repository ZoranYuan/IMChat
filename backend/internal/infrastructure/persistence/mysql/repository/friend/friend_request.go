package friend

import (
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	friendentity "IM_backend/internal/domain/friend/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"errors"

	mysqlDriver "github.com/go-sql-driver/mysql"
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

func (r *FriendRequestRepo) WithTx(tx any) friendrepo.FriendRequestRepository {
	return &FriendRequestRepo{db: tx.(*gorm.DB)}
}

func (fr *FriendRequestRepo) FindLatestRequest(applicantUserId, peerUserId string) (*friendentity.FriendRequest, error) {
	var friendRequestModel model.FriendRequest
	err := fr.db.
		Where("applicant_user_id = ? AND peer_user_id = ?", applicantUserId, peerUserId).
		Order("created_at DESC").
		First(&friendRequestModel).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return requestToDomain(friendRequestModel), nil
}

func (fr *FriendRequestRepo) Create(domain *friendentity.FriendRequest) (*friendentity.FriendRequest, error) {
	m := requestToModel(domain)

	if err := fr.db.Create(&m).Error; err != nil {
		if isDuplicateKey(err) {
			return nil, friendentity.ErrRequestSentTooFrequently
		}
		return nil, err
	}

	return requestToDomain(m), nil
}

func (fr *FriendRequestRepo) ReRequest(domain *friendentity.FriendRequest) error {
	m := requestToModel(domain)

	result := fr.db.Model(&model.FriendRequest{}).
		Where(
			"applicant_user_id = ? AND peer_user_id = ?",
			m.ApplicantUserId,
			m.PeerUserId,
		).
		Updates(map[string]interface{}{
			"request_id": m.RequestId,
			"status":     m.Status,
			"message":    m.Message,
			"apply_time": m.ApplyTime,
		})

	if result.Error != nil {
		if isDuplicateKey(result.Error) {
			return friendentity.ErrRequestSentTooFrequently
		}
		return result.Error
	}

	if result.RowsAffected == 0 {
		return friendentity.ErrInvalidStatusTransition
	}

	return nil
}

func isDuplicateKey(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
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
		return friendentity.ErrInvalidStatusTransition
	}

	return nil
}

func (fr *FriendRequestRepo) ListByUserID(peerUserId string) ([]*friendentity.FriendRequest, error) {
	var m []model.FriendRequest
	if err := fr.db.Where("peer_user_id = ?", peerUserId).
		Order("updated_at DESC").
		Find(&m).Error; err != nil {
		return nil, err
	}

	domainList := make([]*friendentity.FriendRequest, 0, len(m))

	for _, m := range m {
		domainList = append(domainList, requestToDomain(m))
	}

	return domainList, nil
}

func (fr *FriendRequestRepo) FindByRequestID(requestId string) (*friendentity.FriendRequest, error) {
	var m model.FriendRequest

	if err := fr.db.Where("request_id = ?", requestId).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, friendentity.ErrFriendRequestNotFound
		}
		return nil, err
	}

	return requestToDomain(m), nil
}
