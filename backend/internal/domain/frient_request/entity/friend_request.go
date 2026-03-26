package friend_request_entity

import (
	friend_request_valueobject "IM_backend/internal/domain/frient_request/value_object"
	"time"
)

type FriendRequest struct {
	FromUserId string                            `json:"fromUserId"`
	ToUserId   string                            `json:"toUserId"`
	Status     friend_request_valueobject.Status `json:"status"`
	Message    string                            `json:"message"` // 可选留言
	ApplyTime  time.Time                         `json:"applyTime"`
}

func NewFriendRequest(fromUserId, toUserId, message string) *FriendRequest {
	// TODO 对 message 做一些额外的处理

	var newFriendRequest = &FriendRequest{
		FromUserId: fromUserId,
		ToUserId:   toUserId,
		Message:    message,
		Status:     friend_request_valueobject.Pedding,
		ApplyTime:  time.Now(),
	}

	return newFriendRequest
}

func (fq *FriendRequest) ReApply(message string) error {
	if time.Since(fq.ApplyTime) < 10*time.Minute {
		return ErrApplyingTooFrequently
	}

	fq.Message = message
	fq.Status = friend_request_valueobject.Pedding
	fq.ApplyTime = time.Now()

	return nil
}
