package friend

import (
	friendentity "IM_backend/internal/domain/friend/entity"
)

// TODO 定义 usermysql 的接口
type FriendRequestRepository interface {
	FindLatestRequest(userId string, toUserId string) (*friendentity.FriendRequest, error)
	Create(domain *friendentity.FriendRequest) (*friendentity.FriendRequest, error)
	OperateRequest(requestId string, expectStatus, newStatus int) error
	ReRequest(domain *friendentity.FriendRequest) error
	ListByUserID(userId string) ([]*friendentity.FriendRequest, error)
	FindByRequestID(requestId string) (*friendentity.FriendRequest, error)
	WithTx(tx any) FriendRequestRepository
}
