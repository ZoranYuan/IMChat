package user

import "errors"

var (
	ErrEmptyUserId  = errors.New("用户标识不能为空")
	ErrUserNotFonud = errors.New("用户不存在")
	ErrInvalidTTL   = errors.New("缓存有效期必须大于零")
)
