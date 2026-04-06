package application_message

import "errors"

var (
	// 消息存储失败
	ErrMessageSave = errors.New("message save failed")

	// 消息查询失败
	ErrMessageQuery = errors.New("message query failed")

	// 消息不存在
	ErrMessageNotFound = errors.New("message not found")

	// 会话创建失败
	ErrConversationCreate = errors.New("conversation create failed")

	// 会话查询失败
	ErrConversationQuery = errors.New("conversation query failed")

	// 会话不存在
	ErrConversationNotFound = errors.New("conversation not found")

	// 更新 seq 失败
	ErrConversationUpdateSeq = errors.New("conversation update last seq failed")

	// 用户会话创建失败
	ErrUserConversationCreate = errors.New("user conversation create failed")

	// 用户会话查询失败
	ErrUserConversationQuery = errors.New("user conversation query failed")

	// 更新已读失败
	ErrUserConversationUpdateReadSeq = errors.New("user conversation update read seq failed")

	// 用户会话不存在
	ErrUserConversationNotFound = errors.New("user conversation not found")

	ErrUnknown = errors.New("unknwon err")
)
