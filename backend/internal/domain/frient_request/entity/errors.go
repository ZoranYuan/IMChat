package friend_request_entity

import "errors"

var (
	ErrApplyingTooFrequently   = errors.New("申请太频繁")
	ErrInvalidStatusTransition = errors.New("非法状态转换")
	ErrDuplicateOperation      = errors.New("无效操作")
	ErrAlreadyFriend           = errors.New("已经是好友了")
	ErrInvalidateStatus        = errors.New("非法状态")
	ErrRequestSelf             = errors.New("无法向自己发送请求")
	ErrInvalidateOperate       = errors.New("非法操作")
)
