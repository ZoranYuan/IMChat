package friend_entity

import "errors"

var (
	ErrCannotAddSelf = errors.New("无法添加自己为好友")
)
