package application_room

import room_entity "IM_backend/internal/domain/room/entity"

type RoomAppDTO struct {
	RoomId      string
	OwnerUserId string
	Description string
	RoomName    string
	Status      int
	Avatar      string
	MemberCount int
	MaxMembers  int
	InviteCode  string
}

func toDTO(r *room_entity.Room, inviteCode string) *RoomAppDTO {
	return &RoomAppDTO{
		RoomId:      r.RoomId,
		OwnerUserId: r.OwnerUserId,
		Description: r.Description,
		RoomName:    r.RoomName,
		Status:      int(r.Status),
		Avatar:      r.Avatar,
		MemberCount: r.MemberCount,
		MaxMembers:  r.MaxMembers,
		InviteCode:  inviteCode,
	}
}
