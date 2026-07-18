package message

import (
	friendentity "IM_backend/internal/domain/friend/entity"
	roomentity "IM_backend/internal/domain/room/entity"
	"errors"
)

var (
	ErrUnknownConversationType = errors.New("未知的会话类型")
	ErrMessageSave             = errors.New("保存消息失败")

	ErrConversationNotFound = errors.New("会话不存在")

	ErrNotFriends = friendentity.ErrNotFriends

	ErrNotRoomMember = roomentity.ErrMemberNotFound

	ErrConversationSequenceUpdate = errors.New("更新会话序列失败")

	ErrForbidden = errors.New("无权发送消息")

	ErrUnknown = errors.New("未知错误")
)
