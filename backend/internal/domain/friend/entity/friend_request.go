package entity

import (
	friendvo "IM_backend/internal/domain/friend/value_object"
	"time"
)

type FriendRequest struct {
	RequestId  string                 `json:"requestId"`
	FromUserId string                 `json:"fromUserId"`
	ToUserId   string                 `json:"toUserId"`
	Status     friendvo.RequestStatus `json:"status"`
	Message    string                 `json:"message"` // 可选留言
	ApplyTime  int64                  `json:"applyTime"`
}

func NewFriendRequest(reqId, fromUserId, toUserId, message string) (*FriendRequest, error) {
	// TODO 对 message 做一些额外的处理
	if fromUserId == toUserId {
		return nil, ErrSelfRequest
	}

	var newFriendRequest = &FriendRequest{
		FromUserId: fromUserId,
		RequestId:  reqId,
		ToUserId:   toUserId,
		Message:    message,
		Status:     friendvo.Pending,
		ApplyTime:  time.Now().UnixMilli(),
	}

	return newFriendRequest, nil
}

func (fq *FriendRequest) ReRequest(newRequestId string, message string) error {
	if fq.FromUserId == fq.ToUserId {
		return ErrSelfRequest
	}

	if time.Since(time.UnixMilli(fq.ApplyTime)) < 10*time.Minute {
		return ErrRequestSentTooFrequently
	}

	fq.RequestId = newRequestId
	fq.Message = message
	fq.Status = friendvo.Pending
	fq.ApplyTime = time.Now().UnixMilli()

	return nil
}

func (fq *FriendRequest) Accept(userId string) error {
	if fq.Status != friendvo.Pending {
		return ErrDuplicateRequestOperation
	}

	if fq.ToUserId != userId {
		return ErrInvalidRequestOperation
	}

	fq.Status = friendvo.Accepted

	return nil
}

func (fq *FriendRequest) Refuse(userId string) error {
	var err error
	if fq.Status != friendvo.Pending {
		return ErrDuplicateRequestOperation
	}

	if fq.ToUserId != userId {
		return ErrInvalidRequestOperation
	}

	fq.Status = friendvo.Refused

	return err
}
