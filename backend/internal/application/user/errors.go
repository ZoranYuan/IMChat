package user

import (
	userentity "IM_backend/internal/domain/user/entity"
	"errors"
)

var (
	ErrUserAlreadyExists  = userentity.ErrUserAlreadyExists
	ErrUserNotFound       = userentity.ErrUserNotFound
	ErrIncorrectPassword  = userentity.ErrIncorrectPassword
	ErrInvalidPhoneNumber = userentity.ErrInvalidPhoneNumber
	ErrPasswordMismatch   = errors.New("两次输入的密码不一致")
)
