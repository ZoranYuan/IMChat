package room_repository

import (
	room_entity "IM_backend/internal/domain/room/entity"
	room_valueobject "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/database/mysql/model"
)

func toDomain(m model.Room) *room_entity.Room {
	return &room_entity.Room{
		RoomId:      m.RoomId,
		OwnerUserId: m.OwnerUserId,
		Description: m.Description,
		RoomName:    m.RoomName,
		Status:      room_valueobject.RoomStatus(m.Status),
		Avatar:      m.Avatar,
		MemberCount: m.MemberCount,
		MaxMembers:  m.MaxMembers,
	}
}

func toModel(e *room_entity.Room) model.Room {
	return model.Room{
		RoomId:      e.RoomId,
		OwnerUserId: e.OwnerUserId,
		Description: e.Description,
		RoomName:    e.RoomName,
		Status:      int(e.Status),
		Avatar:      e.Avatar,
		MemberCount: e.MemberCount,
		MaxMembers:  e.MaxMembers,
	}
}
