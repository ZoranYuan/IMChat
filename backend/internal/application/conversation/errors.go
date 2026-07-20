package conversation

import "errors"

var (
	ErrEmptyUserId           = errors.New("用户标识不能为空")
	ErrNoConversationsRecord = errors.New("暂无会话记录")
)
