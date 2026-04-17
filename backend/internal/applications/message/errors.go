package application_message

import "errors"

var (
	ErrMessageSave = errors.New("message save failed")

	ErrMessageQuery = errors.New("message query failed")

	ErrMessageNotFound = errors.New("message not found")

	ErrConversationCreate = errors.New("conversation create failed")

	ErrConversationQuery = errors.New("conversation query failed")

	ErrConversationNotFound = errors.New("conversation not found")

	ErrNotFriend = errors.New("not friend")

	ErrNotInRoom = errors.New("not in the room")

	ErrConversationUpdateSeq = errors.New("conversation update last seq failed")

	ErrUserConversationCreate = errors.New("user conversation create failed")

	ErrUserConversationQuery = errors.New("user conversation query failed")

	ErrUserConversationUpdateReadSeq = errors.New("user conversation update read seq failed")

	ErrUserConversationNotFound = errors.New("user conversation not found")

	ErrForbidden = errors.New("user forbidden")

	ErrUnknown = errors.New("unknwon err")
)
