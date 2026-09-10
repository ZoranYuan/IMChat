package entity

import (
	friendvo "IM_backend/internal/domain/friend/value_object"
	"time"
)

type FriendRequest struct {
	RequestId       string                 `json:"requestId"`
	ApplicantUserId string                 `json:"applicantUserId"`
	TargetUserId    string                 `json:"targetUserId"`
	Status          friendvo.RequestStatus `json:"status"`
	Message         string                 `json:"message"` // 可选留言
	ApplyTime       int64                  `json:"applyTime"`
}

func NewFriendRequest(reqId, applicantUserId, targetUserId, message string) (*FriendRequest, error) {
	// TODO 对 message 做一些额外的处理
	if applicantUserId == targetUserId {
		return nil, ErrSelfRequest
	}

	var newFriendRequest = &FriendRequest{
		ApplicantUserId: applicantUserId,
		RequestId:       reqId,
		TargetUserId:    targetUserId,
		Message:         message,
		Status:          friendvo.Pending,
		ApplyTime:       time.Now().UnixMilli(),
	}

	return newFriendRequest, nil
}

func (fq *FriendRequest) ReRequest(newRequestId string, message string) error {
	if fq.ApplicantUserId == fq.TargetUserId {
		return ErrSelfRequest
	}

	// 查询时间是否超过重新请求阈值
	if time.Since(time.UnixMilli(fq.ApplyTime)) < 10*time.Minute {
		return ErrRequestSentTooFrequently
	}

	// 上次请求未处理，视为同一个请求
	if fq.Status == friendvo.Pending {
		fq.Message = message
		fq.ApplyTime = time.Now().UnixMilli()
		return nil
	}

	// 重新请求需要重新去建立新的 request id
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

	if fq.TargetUserId != userId {
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

	if fq.TargetUserId != userId {
		return ErrInvalidRequestOperation
	}

	fq.Status = friendvo.Refused

	return err
}
