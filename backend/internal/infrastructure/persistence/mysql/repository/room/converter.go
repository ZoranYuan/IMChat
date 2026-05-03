package room

import (
	roomentity "IM_backend/internal/domain/room/entity"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func toDomain(m model.Room) *roomentity.Room {
	return &roomentity.Room{
		RoomId:      m.RoomId,
		OwnerUserId: m.OwnerUserId,
		Description: m.Description,
		RoomName:    m.RoomName,
		Status:      roomvo.RoomStatus(m.Status),
		Avatar:      m.Avatar,
		MemberCount: m.MemberCount,
		MaxMembers:  m.MaxMembers,
		Version:     m.Version,
	}
}

func toModel(e *roomentity.Room) model.Room {
	return model.Room{
		RoomId:      e.RoomId,
		OwnerUserId: e.OwnerUserId,
		Description: e.Description,
		RoomName:    e.RoomName,
		Status:      int(e.Status),
		Avatar:      e.Avatar,
		MemberCount: e.MemberCount,
		MaxMembers:  e.MaxMembers,
		Version:     e.Version,
	}
}
