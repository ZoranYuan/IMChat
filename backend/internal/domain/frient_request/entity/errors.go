package friend_request_entity

import "errors"

var (
	ErrApplyingTooFrequently   = errors.New("申请太频繁")
	ErrInvalidStatusTransition = errors.New("非法状态转换")
)
