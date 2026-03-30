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
	ApplyTime  int64                             `json:"applyTime"`
}

func NewFriendRequest(reqId, fromUserId, toUserId, message string) (*FriendRequest, error) {
	// TODO 对 message 做一些额外的处理
	if fromUserId == toUserId {
		return nil, ErrRequestSelf
	}

	var newFriendRequest = &FriendRequest{
		FromUserId: fromUserId,
		RequestId:  reqId,
		ToUserId:   toUserId,
		Message:    message,
		Status:     friend_request_valueobject.Pending,
		ApplyTime:  time.Now().Unix(),
	}

	return newFriendRequest, nil
}

func (fq *FriendRequest) ReRequest(message string) error {
	if fq.FromUserId == fq.ToUserId {
		return ErrRequestSelf
	}

	if time.Duration(time.Now().Unix()-fq.ApplyTime)*time.Second < 10*time.Minute {
		return ErrApplyingTooFrequently
	}

	if fq.Status != friend_request_valueobject.Pending {
		return ErrInvalidateStatus
	}

	fq.Message = message
	fq.Status = friend_request_valueobject.Pending
	fq.ApplyTime = time.Now().Unix()

	return nil
}

func (fq *FriendRequest) Accept(userId string) error {
	if fq.Status != friend_request_valueobject.Pending {
		return ErrDuplicateOperation
	}

	if fq.ToUserId != userId {
		return ErrInvalidateOperate
	}

	fq.Status = friend_request_valueobject.Accepted

	return nil
}

func (fq *FriendRequest) Refuse(userId string) error {
	var err error
	if fq.Status != friend_request_valueobject.Pending {
		return ErrDuplicateOperation
	}

	if fq.ToUserId != userId {
		return ErrInvalidateOperate
	}

	fq.Status = friend_request_valueobject.Refused

	return err
}
