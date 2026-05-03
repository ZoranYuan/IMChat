package application_room

import "errors"

var (
	ErrRoomNameRequired  = errors.New("room name is required")
	ErrRoomNotFound      = errors.New("room not found")
	ErrUnknown           = errors.New("unknown error")
	ErrNotRoomMember     = errors.New("user is not a room member")
	ErrRoomUnavailable   = errors.New("room is unavailable")
	ErrInviteCodeExpired = errors.New("invite code has expired")
	ErrConcurrentUpdate  = errors.New("concurrent update detected")
	ErrPermissionDenied  = errors.New("permission denied")
)
