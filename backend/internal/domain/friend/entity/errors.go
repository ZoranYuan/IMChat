package friend_entity

import "errors"

var (
	ErrCannotAddSelf = errors.New("cannot add yourself as a friend")
)
