package entity

import (
	roomvo "IM_backend/internal/domain/room/value_object"
	"time"
)

type RoomUser struct {
	UserId    string
	RoomId    string
	Role      roomvo.Role
	Status    roomvo.RoomUserStatus
	MuteUtil  *int64
	JoinTime  int64
	LeaveTime *int64

	// 标识用户在某个房间中的成员状态版本
	Version int64
}

func NewRoomUser(userId, roomId string, role roomvo.Role) *RoomUser {
	return &RoomUser{
		UserId:   userId,
		RoomId:   roomId,
		Role:     role,
		Status:   roomvo.Activate,
		MuteUtil: nil,
		JoinTime: time.Now().UnixMilli(),
		Version:  1,
	}
}

func (ru *RoomUser) Invite() error {
	if ru.Status != roomvo.Activate {
		return ErrPermissionDenied
	}

	return nil
}

func (ru *RoomUser) Join() {
	ru.Status = roomvo.Activate
	ru.JoinTime = time.Now().UnixMilli()
	ru.MuteUtil = nil
	ru.LeaveTime = nil
}

func (ru *RoomUser) ReJoin() error {
	if ru.Status != roomvo.BeKicked && ru.Status != roomvo.Left {
		return ErrDuplicateJoin
	}

	ru.Status = roomvo.Activate
	ru.JoinTime = time.Now().UnixMilli()
	ru.MuteUtil = nil
	ru.LeaveTime = nil
	return nil
}

func (ru *RoomUser) Leave() error {
	if ru.Status != roomvo.BeMuted && ru.Status != roomvo.Activate {
		return ErrDuplicateLeave
	}
	ru.Status = roomvo.Left
	now := time.Now().UnixMilli()
	ru.LeaveTime = &now

	return nil
}
