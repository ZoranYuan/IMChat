package application_friend_request

import "errors"

var (
	ErrApplyingTooFrequently   = errors.New("申请太频繁")
	ErrAlreadyFriend           = errors.New("已经是好友了")
	ErrInvalidStatusTransition = errors.New("非法状态转换")
	ErrDuplicateOperation      = errors.New("无效操作")
	ErrRequestSelf             = errors.New("无效的申请")
	ErrUnknown                 = errors.New("未知错误")
	ErrOperateFailed           = errors.New("操作失败")
)
