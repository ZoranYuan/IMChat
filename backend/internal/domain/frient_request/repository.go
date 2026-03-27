package friend_request

import (
	friend_request_entity "IM_backend/internal/domain/frient_request/entity"
)

// TODO 定义 user_repository 的接口
type FriendRequestInterface interface {
	FindByUsers(userId string, toUserId string) (*friend_request_entity.FriendRequest, error)
	Create(domain *friend_request_entity.FriendRequest) (*friend_request_entity.FriendRequest, error)
	OperateRequest(requestId string, expectStatus, newStatus int) error
	ReRequest(domain *friend_request_entity.FriendRequest) error
	ListByUserId(userId string) ([]*friend_request_entity.FriendRequest, error)
	FindByRequestId(requestId string) (*friend_request_entity.FriendRequest, error)
}
