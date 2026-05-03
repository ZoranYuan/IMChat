package application_friend_request

import (
	friend_request_entity "IM_backend/internal/domain/friend_request/entity"
)

type FriendRequestDTO struct {
	RequestId  string `json:"requestId"`
	FromUserId string `json:"fromUserId"`
	ToUserId   string `json:"toUserId"`
	Message    string `json:"message"`
	Status     int    `json:"status"`
	ApplyTime  int64  `json:"applyTime"`
}

func toDTO(fr *friend_request_entity.FriendRequest) FriendRequestDTO {
	return FriendRequestDTO{
		RequestId:  fr.RequestId,
		FromUserId: fr.FromUserId,
		ToUserId:   fr.ToUserId,
		Message:    fr.Message,
		Status:     int(fr.Status),
		ApplyTime:  fr.ApplyTime,
	}
}
