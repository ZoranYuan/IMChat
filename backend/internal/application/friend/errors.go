package friend

import (
	friendentity "IM_backend/internal/domain/friend/entity"
	userentity "IM_backend/internal/domain/user/entity"
	"errors"
)

var (
	ErrRequestSentTooFrequently = friendentity.ErrRequestSentTooFrequently
	ErrAlreadyFriends           = friendentity.ErrAlreadyFriends
	ErrInvalidStatusTransition  = friendentity.ErrInvalidStatusTransition
	ErrDuplicateOperation       = friendentity.ErrDuplicateRequestOperation
	ErrSelfRequest              = friendentity.ErrSelfRequest
	ErrUserNotFound             = userentity.ErrUserNotFound
	ErrEmptyUserId              = errors.New("用户 ID 不能为空")
	ErrOperationFailed          = errors.New("好友申请操作失败")
	ErrUnknown                  = errors.New("未知错误")
	ErrCreateConvFailed         = errors.New("创建会话失败")
	ErrCreateUserConvFailed     = errors.New("创建用户会话失败")
	ErrTooManyMessage           = errors.New("好友申请附言不能超过 200 字")
)
