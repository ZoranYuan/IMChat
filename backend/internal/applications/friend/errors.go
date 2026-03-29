package application_friend

import "errors"

var (
	ErrAlreadyFriend = errors.New("你们已经是好友，无法重复添加")
)
