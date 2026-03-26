package friend_request_repository

import (
	friend_request_entity "IM_backend/internal/domain/frient_request/entity"
	friend_request_valueobject "IM_backend/internal/domain/frient_request/value_object"
	"IM_backend/internal/infrastructure/database/mysql/model"
)

func toDomain(m *model.FriendRequest) *friend_request_entity.FriendRequest {
	return &friend_request_entity.FriendRequest{
		FromUserId: m.FromUserId,
		ToUserId:   m.ToUserId,
		Message:    m.Message,
		Status:     friend_request_valueobject.Status(m.Status),
		ApplyTime:  m.UpdatedAt,
	}
}

func toModel(e *friend_request_entity.FriendRequest) *model.FriendRequest {
	return &model.FriendRequest{
		FromUserId: e.FromUserId,
		ToUserId:   e.ToUserId,
		Message:    e.Message,
		Status:     int(e.Status),
	}
}
