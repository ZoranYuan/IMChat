package friend_request_entity

import (
	friend_request_valueobject "IM_backend/internal/domain/frient_request/value_object"
	"time"
)

type FriendRequest struct {
	RequestId  string                            `json:"requestId"`
	FromUserId string                            `json:"fromUserId"`
	ToUserId   string                            `json:"toUserId"`
	Status     friend_request_valueobject.Status `json:"status"`
	Message    string                            `json:"message"` // 可选留言
	ApplyTime  time.Time                         `json:"applyTime"`
}

func NewFriendRequest(reqId, fromUserId, toUserId, message string) *FriendRequest {
	// TODO 对 message 做一些额外的处理
	var newFriendRequest = &FriendRequest{
		FromUserId: fromUserId,
		RequestId:  reqId,
		ToUserId:   toUserId,
		Message:    message,
		Status:     friend_request_valueobject.Pending,
		ApplyTime:  time.Now(),
	}

	return newFriendRequest
}

func (fq *FriendRequest) ReRequest(message string) error {
	if time.Since(fq.ApplyTime) < 10*time.Minute {
		return ErrApplyingTooFrequently
	}

	fq.Message = message
	fq.Status = friend_request_valueobject.Pending
	fq.ApplyTime = time.Now()

	return nil
}

func (fq *FriendRequest) Accept() error {
	var err error
	if fq.Status != friend_request_valueobject.Pending {
		return ErrDuplicateOperation
	}

	fq.Status = friend_request_valueobject.Accepted

	return err
}

func (fq *FriendRequest) Refuse() error {
	var err error
	if fq.Status != friend_request_valueobject.Pending {
		return ErrDuplicateOperation
	}

	fq.Status = friend_request_valueobject.Refused

	return err
}
