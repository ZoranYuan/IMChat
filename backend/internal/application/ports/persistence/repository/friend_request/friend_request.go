package friendrequest

import (
	friendrequestentity "IM_backend/internal/domain/friend_request/entity"

	"gorm.io/gorm"
)

// TODO 定义 usermysql 的接口
type FriendRequestRepository interface {
	FindLatestRequest(userId string, toUserId string) (*friendrequestentity.FriendRequest, error)
	Create(domain *friendrequestentity.FriendRequest) (*friendrequestentity.FriendRequest, error)
	OperateRequest(requestId string, expectStatus, newStatus int) error
	ReRequest(domain *friendrequestentity.FriendRequest) error
	ListByUserID(userId string) ([]*friendrequestentity.FriendRequest, error)
	FindByRequestID(requestId string) (*friendrequestentity.FriendRequest, error)
	WithTx(tx *gorm.DB) FriendRequestRepository
}
