package room

import roomentity "IM_backend/internal/domain/room/entity"

type RoomAppDTO struct {
	RoomID      string
	OwnerUserID string
	Description string
	RoomName    string
	Status      int
	UserRole    int
	AvatarURL   string
	MemberCount int
	MaxMembers  int
	InviteCode  string
}

type RoomUserDTO struct {
	UserID    string
	RoomID    string
	Role      int
	Status    int
	MuteUntil *int64 // 禁言到什么时候（时间戳）
	JoinTime  int64
	LeaveTime *int64
}

func toRoomAppDTO(r *roomentity.Room, inviteCode string) *RoomAppDTO {
	return &RoomAppDTO{
		RoomID:      r.RoomId,
		OwnerUserID: r.OwnerUserId,
		Description: r.Description,
		RoomName:    r.RoomName,
		Status:      int(r.Status),
		AvatarURL:   r.Avatar,
		MemberCount: r.MemberCount,
		MaxMembers:  r.MaxMembers,
		InviteCode:  inviteCode,
	}
}

func toRoomUserDTO(r *roomentity.RoomUser) *RoomUserDTO {
	return &RoomUserDTO{
		UserID:    r.UserId,
		RoomID:    r.RoomId,
		Role:      int(r.Role),
		Status:    int(r.Status),
		MuteUntil: r.MuteUtil,
		JoinTime:  r.JoinTime,
		LeaveTime: r.LeaveTime,
	}
}
