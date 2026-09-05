package friend

import (
	friendrequestentity "IM_backend/internal/domain/friend/entity"
)

type FriendRequestDTO struct {
	RequestID       string
	FromUserID      string
	FromUsername    string
	FromDisplayName string
	ToUserID        string
	Message         string
	Status          int
	ApplyTime       int64
}

func toFriendRequestDTO(fr *friendrequestentity.FriendRequest) FriendRequestDTO {
	return FriendRequestDTO{
		RequestID:  fr.RequestId,
		FromUserID: fr.FromUserId,
		ToUserID:   fr.ToUserId,
		Message:    fr.Message,
		Status:     int(fr.Status),
		ApplyTime:  fr.ApplyTime,
	}
}
