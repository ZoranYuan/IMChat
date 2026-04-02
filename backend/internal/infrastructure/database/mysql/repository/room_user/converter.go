package room_user_repository

import (
	room_entity "IM_backend/internal/domain/room/entity"
	room_valueobject "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/database/mysql/model"
)

func ToDomain(m model.RoomUser) *room_entity.RoomUser {
	return &room_entity.RoomUser{
		UserId:   m.UserId,
		RoomId:   m.RoomId,
		Role:     room_valueobject.Role(m.Role),
		Status:   room_valueobject.RoomUserStatus(m.Status),
		MuteUtil: m.MuteUtil,
		JoinTime: m.JoinTime,
		Version:  m.Version,
	}
}

func ToModel(e *room_entity.RoomUser) model.RoomUser {
	return model.RoomUser{
		UserId:   e.UserId,
		RoomId:   e.RoomId,
		Role:     int(e.Role),
		Status:   int(e.Status),
		MuteUtil: e.MuteUtil,
		JoinTime: e.JoinTime,
		Version:  e.Version,
	}
}
