package friend

import (
	friendentity "IM_backend/internal/domain/friend/entity"
	friendvo "IM_backend/internal/domain/friend/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func toDomain(m model.Friend) friendentity.Friend {
	return friendentity.Friend{
		UserId:       m.UserId,
		FriendUserId: m.FriendUserId,
		Status:       friendvo.Status(m.Status),
		Remarks:      m.Remarks,
	}
}

func toModel(e friendentity.Friend) model.Friend {
	return model.Friend{
		UserId:       e.UserId,
		FriendUserId: e.FriendUserId,
		Status:       int(e.Status),
		Remarks:      e.Remarks,
	}
}
