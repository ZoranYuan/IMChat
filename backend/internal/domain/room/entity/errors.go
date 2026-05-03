package room_entity

import "errors"

var (
	ErrDuplicateCreation = errors.New("duplicate creation")
	ErrDuplicateJoin     = errors.New("duplicate join")
	ErrDuplicateLeave    = errors.New("duplicate leave")
	ErrInviteCodeExpired = errors.New("invite code has expired")
	ErrRoomNameRequired  = errors.New("room name is required")
	ErrRoomNotFound      = errors.New("room not found")
	ErrMemberNotFound    = errors.New("room membership not found")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrVersionConflict   = errors.New("version conflict")
)
