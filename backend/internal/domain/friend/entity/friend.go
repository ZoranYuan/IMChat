package friend_entity

import friend_valueobject "IM_backend/internal/domain/friend/value_object"

type Friend struct {
	UserId       string                    `json:"userId" `
	FriendUserId string                    `json:"friendUserId"`
	UserName     string                    `json:"userName"`
	NickName     string                    `json:"nickName"`
	Remarks      string                    `json:"remarks"`
	Status       friend_valueobject.Status `json:"status"`
}
