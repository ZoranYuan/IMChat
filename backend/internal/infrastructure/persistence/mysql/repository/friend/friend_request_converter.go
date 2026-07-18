package friend

import (
	friendentity "IM_backend/internal/domain/friend/entity"
	friendvo "IM_backend/internal/domain/friend/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func requestToDomain(m model.FriendRequest) *friendentity.FriendRequest {
	return &friendentity.FriendRequest{
		FromUserId: m.FromUserId,
		ToUserId:   m.ToUserId,
		RequestId:  m.RequestId,
		Message:    m.Message,
		Status:     friendvo.RequestStatus(m.Status),
		ApplyTime:  m.ApplyTime,
	}
}

func requestToModel(e *friendentity.FriendRequest) model.FriendRequest {
	return model.FriendRequest{
		FromUserId: e.FromUserId,
		RequestId:  e.RequestId,
		ToUserId:   e.ToUserId,
		Message:    e.Message,
		Status:     int(e.Status),
		ApplyTime:  e.ApplyTime,
	}
}
