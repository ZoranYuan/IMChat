package entity

import friendvo "IM_backend/internal/domain/friend/value_object"

type Friend struct {
	UserId       string
	FriendUserId string
	Remarks      string
	Status       friendvo.Status
}
