package friend_repository

import (
	friend_entity "IM_backend/internal/domain/friend/entity"
	friend_valueobject "IM_backend/internal/domain/friend/value_object"
	"IM_backend/internal/infrastructure/database/mysql/model"
)

func toDomain(m model.Friend) friend_entity.Friend {
	return friend_entity.Friend{
		UserId:       m.UserId,
		FriendUserId: m.FriendUserId,
		Status:       friend_valueobject.Status(m.Status),
		Remarks:      m.Remarks,
	}
}

func toModel(e friend_entity.Friend) model.Friend {
	return model.Friend{
		UserId:       e.UserId,
		FriendUserId: e.FriendUserId,
		Status:       int(e.Status),
		Remarks:      e.Remarks,
	}
}
