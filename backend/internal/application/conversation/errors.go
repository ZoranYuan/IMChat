package conversation

import "errors"

var (
	ErrEmptyUserId           = errors.New("userId 不能为空")
	ErrNoConversationsRecord = errors.New("暂无会话记录")
)
