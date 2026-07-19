package user

import "errors"

var (
	ErrEmptyUserId  = errors.New("userId 不为空")
	ErrUserNotFonud = errors.New("用户不存在")
	ErrInvalidTTL   = errors.New("缓存有效期必须大于零")
)
