package friendrequest

import (
	friendrequestentity "IM_backend/internal/domain/friend_request/entity"
	friendrequestvo "IM_backend/internal/domain/friend_request/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func toDomain(m model.FriendRequest) *friendrequestentity.FriendRequest {
	return &friendrequestentity.FriendRequest{
		FromUserId: m.FromUserId,
		ToUserId:   m.ToUserId,
		RequestId:  m.RequestId,
		Message:    m.Message,
		Status:     friendrequestvo.Status(m.Status),
		ApplyTime:  m.ApplyTime,
	}
}

func toModel(e *friendrequestentity.FriendRequest) model.FriendRequest {
	return model.FriendRequest{
		FromUserId: e.FromUserId,
		RequestId:  e.RequestId,
		ToUserId:   e.ToUserId,
		Message:    e.Message,
		Status:     int(e.Status),
		ApplyTime:  e.ApplyTime,
	}
}
