package room_entity

import (
	room_valueobject "IM_backend/internal/domain/room/value_object"
	"time"
)

type RoomUser struct {
	UserId    string
	RoomId    string
	Role      room_valueobject.Role
	Status    room_valueobject.RoomUserStatus
	MuteUtil  *int64
	JoinTime  int64
	LeaveTime *int64
	Version   int64
}

func NewRoomUser(userId, roomId string, role room_valueobject.Role) *RoomUser {
	return &RoomUser{
		UserId:   userId,
		RoomId:   roomId,
		Role:     role,
		Status:   room_valueobject.Activate,
		MuteUtil: nil,
		JoinTime: time.Now().UnixMilli(),
		Version:  1,
	}
}

func (ru *RoomUser) Invite() error {
	if ru.Status != room_valueobject.Activate {
		return ErrPermissionDenied
	}

	return nil
}

func (ru *RoomUser) Join() {
	ru.Status = room_valueobject.Activate
	ru.JoinTime = time.Now().UnixMilli()
	ru.MuteUtil = nil
	ru.LeaveTime = nil
}

func (ru *RoomUser) ReJoin() error {
	if ru.Status != room_valueobject.BeKicked && ru.Status != room_valueobject.Left {
		return ErrDuplicateJoin
	}

	ru.Status = room_valueobject.Activate
	ru.JoinTime = time.Now().UnixMilli()
	ru.MuteUtil = nil
	ru.LeaveTime = nil
	return nil
}

func (ru *RoomUser) Leave() error {
	if ru.Status != room_valueobject.BeMuted && ru.Status != room_valueobject.Activate {
		return ErrDuplicateLeave
	}
	ru.Status = room_valueobject.Left
	now := time.Now().UnixMilli()
	ru.LeaveTime = &now

	return nil
}
