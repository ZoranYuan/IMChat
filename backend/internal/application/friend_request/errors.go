package application_friend_request

import "errors"

var (
	ErrRequestSentTooFrequently = errors.New("friend request sent too frequently")
	ErrAlreadyFriends           = errors.New("users are already friends")
	ErrInvalidStatusTransition  = errors.New("invalid status transition")
	ErrDuplicateOperation       = errors.New("duplicate operation")
	ErrSelfRequest              = errors.New("cannot send a friend request to yourself")
	ErrUserNotFound             = errors.New("user not found")
	ErrOperationFailed          = errors.New("friend request operation failed")
	ErrUnknown                  = errors.New("unknown error")
)
