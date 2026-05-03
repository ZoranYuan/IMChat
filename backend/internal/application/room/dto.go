package application_room

import room_entity "IM_backend/internal/domain/room/entity"

type RoomAppDTO struct {
	RoomId      string
	OwnerUserId string
	Description string
	RoomName    string
	Status      int
	UserRole    int
	Avatar      string
	MemberCount int
	MaxMembers  int
	InviteCode  string
}

type RoomUserDTO struct {
	UserId    string
	RoomId    string
	Role      int
	Status    int
	MuteUtil  *int64 // 禁言到什么时候（时间戳）
	JoinTime  int64
	LeaveTime *int64
}

func toRoomAppDTO(r *room_entity.Room, inviteCode string) *RoomAppDTO {
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

func toRoomUserDTO(r *room_entity.RoomUser) *RoomUserDTO {
	return &RoomUserDTO{
		UserId:    r.UserId,
		RoomId:    r.RoomId,
		Role:      int(r.Role),
		Status:    int(r.Status),
		MuteUtil:  r.MuteUtil,
		JoinTime:  r.JoinTime,
		LeaveTime: r.LeaveTime,
	}
}
