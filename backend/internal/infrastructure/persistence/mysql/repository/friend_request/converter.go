package friend_request_repository

import (
	friend_request_entity "IM_backend/internal/domain/friend_request/entity"
	friend_request_valueobject "IM_backend/internal/domain/friend_request/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func toDomain(m model.FriendRequest) *friend_request_entity.FriendRequest {
	return &friend_request_entity.FriendRequest{
		FromUserId: m.FromUserId,
		ToUserId:   m.ToUserId,
		RequestId:  m.RequestId,
		Message:    m.Message,
		Status:     friend_request_valueobject.Status(m.Status),
		ApplyTime:  m.ApplyTime,
	}
}

func toModel(e *friend_request_entity.FriendRequest) model.FriendRequest {
	return model.FriendRequest{
		FromUserId: e.FromUserId,
		RequestId:  e.RequestId,
		ToUserId:   e.ToUserId,
		Message:    e.Message,
		Status:     int(e.Status),
		ApplyTime:  e.ApplyTime,
	}
}
