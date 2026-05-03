package application_message

import "errors"

var (
	ErrMessageSave = errors.New("failed to save message")

	ErrMessageQuery = errors.New("failed to query messages")

	ErrMessageNotFound = errors.New("message not found")

	ErrConversationCreate = errors.New("failed to create conversation")

	ErrConversationQuery = errors.New("failed to query conversation")

	ErrConversationNotFound = errors.New("conversation not found")

	ErrNotFriends = errors.New("users are not friends")

	ErrNotRoomMember = errors.New("user is not a room member")

	ErrConversationSequenceUpdate = errors.New("failed to update conversation sequence")

	ErrUserConversationCreate = errors.New("failed to create user conversation")

	ErrUserConversationQuery = errors.New("failed to query user conversation")

	ErrUserConversationReadSequenceUpdate = errors.New("failed to update user conversation read sequence")

	ErrUserConversationNotFound = errors.New("user conversation not found")

	ErrForbidden = errors.New("forbidden")

	ErrUnknown = errors.New("unknown error")
)
