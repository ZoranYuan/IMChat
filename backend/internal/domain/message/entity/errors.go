package message_entity

import "errors"

var (
	ErrDuplicateCreation      = errors.New("duplicate creation")
	ErrConversationNotCreated = errors.New("conversation has not been created")
)
