package user

import (
	userentity "IM_backend/internal/domain/user/entity"
	"errors"
)

var (
	ErrUserAlreadyExists   = userentity.ErrUserAlreadyExists
	ErrUserNotFound        = userentity.ErrUserNotFound
	ErrIncorrectPassword   = userentity.ErrIncorrectPassword
	ErrInvalidPhoneNumber  = userentity.ErrInvalidPhoneNumber
	ErrInvalidLoginAccount = errors.New("登录账号不能为空")
	ErrPasswordMismatch    = errors.New("两次输入的密码不一致")
	ErrNoProfileFields     = errors.New("无用户资料更新字段")
)
