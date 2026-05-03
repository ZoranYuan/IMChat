package friend_request_entity

import "errors"

var (
	ErrRequestSentTooFrequently = errors.New("friend request sent too frequently")
	ErrInvalidStatusTransition  = errors.New("invalid status transition")
	ErrDuplicateOperation       = errors.New("duplicate operation")
	ErrInvalidStatus            = errors.New("invalid status")
	ErrSelfRequest              = errors.New("cannot send a friend request to yourself")
	ErrInvalidOperation         = errors.New("invalid operation")
	ErrFriendRequestNotFound    = errors.New("friend request not found")
)
