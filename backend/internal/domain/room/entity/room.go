package room_entity

import (
	room_valueobject "IM_backend/internal/domain/room/value_object"
)

var (
	MaxMembers = 100
)

type Room struct {
	RoomId      string
	OwnerUserId string
	Description string
	RoomName    string
	Status      room_valueobject.RoomStatus
	Avatar      string
	MemberCount int
	MaxMembers  int
	Version     int64
}

func NewRoom(
	roomId string,
	ownerUserId string,
	description string,
	roomName string,
	avatar string,
) (*Room, error) {
	if roomName == "" {
		return nil, ErrRoomNameRequired
	}

	return &Room{
		RoomId:      roomId,
		OwnerUserId: ownerUserId,
		Description: description,
		RoomName:    roomName,
		Status:      room_valueobject.Normal,
		Avatar:      avatar,
	}, nil
}

func (r *Room) Invite() bool {
	if r.Status != room_valueobject.Normal {
		return false
	}

	return true
}
