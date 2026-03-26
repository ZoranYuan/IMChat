package application_friend

import (
	friend_request_entity "IM_backend/internal/domain/frient_request/entity"
	"time"
)

type friendRequestDTO struct {
	FromUserId string    `json:"fromUserId"`
	ToUserId   string    `json:"toUserId"`
	Message    string    `json:"message"`
	Status     int       `json:"status"`
	ApplyTime  time.Time `json:"applyTime"`
}

func toDTO(fr *friend_request_entity.FriendRequest) *friendRequestDTO {
	return &friendRequestDTO{
		FromUserId: fr.FromUserId,
		ToUserId:   fr.ToUserId,
		Message:    fr.Message,
		Status:     int(fr.Status),
		ApplyTime:  fr.ApplyTime,
	}
}
