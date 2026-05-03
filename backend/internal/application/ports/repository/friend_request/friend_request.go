package friend_request_repository_interface

import (
	friend_request_entity "IM_backend/internal/domain/friend_request/entity"

	"gorm.io/gorm"
)

// TODO 定义 user_repository 的接口
type FriendRequestRepository interface {
	FindLatestRequest(userId string, toUserId string) (*friend_request_entity.FriendRequest, error)
	Create(domain *friend_request_entity.FriendRequest) (*friend_request_entity.FriendRequest, error)
	OperateRequest(requestId string, expectStatus, newStatus int) error
	ReRequest(domain *friend_request_entity.FriendRequest) error
	ListByUserID(userId string) ([]*friend_request_entity.FriendRequest, error)
	FindByRequestID(requestId string) (*friend_request_entity.FriendRequest, error)
	WithTx(tx *gorm.DB) FriendRequestRepository
}
