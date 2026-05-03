package roomuser

import (
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func ToDomain(m model.RoomUser) *roomentity.RoomUser {
	return &roomentity.RoomUser{
		UserId:   m.UserId,
		RoomId:   m.RoomId,
		Role:     roomvo.Role(m.Role),
		Status:   roomvo.RoomUserStatus(m.Status),
		MuteUtil: m.MuteUtil,
		JoinTime: m.JoinTime,
		Version:  m.Version,
	}
}

func ToModel(e *roomentity.RoomUser) model.RoomUser {
	return model.RoomUser{
		UserId:    e.UserId,
		RoomId:    e.RoomId,
		Role:      int(e.Role),
		Status:    int(e.Status),
		MuteUtil:  e.MuteUtil,
		JoinTime:  e.JoinTime,
		LeaveTime: e.LeaveTime,
		Version:   e.Version,
	}
}
