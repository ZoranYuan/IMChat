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
	MuteUntil int64
	JoinTime  int64
}

func NewRoomUser(userId, roomId string, role room_valueobject.Role) *RoomUser {
	return &RoomUser{
		UserId:    userId,
		RoomId:    roomId,
		Role:      role,
		Status:    room_valueobject.Activate,
		MuteUntil: 0,
		JoinTime:  time.Now().Unix(),
	}
}
