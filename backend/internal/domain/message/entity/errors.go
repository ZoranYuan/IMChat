package message_entity

import "errors"

var (
	ErrDuplicateCreate        = errors.New("重复创建")
	ErrConversationNotCreated = errors.New("会话未创建")
)
