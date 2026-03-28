package application_friend_request

import "errors"

var (
	ErrApplyingTooFrequently   = errors.New("申请太频繁")
	ErrInvalidStatusTransition = errors.New("非法状态转换")
	ErrDuplicateOperation      = errors.New("无效操作")
)
