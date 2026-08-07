package entity

import (
	roomvo "IM_backend/internal/domain/room/value_object"
)

var (
	MaxMembers = 100
)

type Room struct {
	RoomId      string
	OwnerUserId string
	Description string
	RoomName    string
	Status      roomvo.RoomStatus
	Avatar      string
	MemberCount int
	MaxMembers  int
	Version     int
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
		Status:      roomvo.Normal,
		Avatar:      avatar,
		MemberCount: 1,
		Version:     1,
	}, nil
}

func (r *Room) Invite() bool {
	if r.Status != roomvo.Normal {
		return false
	}

	return true
}
