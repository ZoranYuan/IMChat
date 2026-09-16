package message

import (
	friendentity "IM_backend/internal/domain/friend/entity"
	roomentity "IM_backend/internal/domain/room/entity"
	userentity "IM_backend/internal/domain/user/entity"
	"errors"
)

var (
	ErrUnknownConversationType = errors.New("未知的会话类型")
	ErrMessageSave             = errors.New("保存消息失败")

	ErrMessageSeq           = errors.New("错误的消息序号")
	ErrConversationNotFound = errors.New("会话不存在")

	ErrUserConversationNotFound = errors.New("用户会话不存在")
	ErrMessageNotFound          = errors.New("消息不存在")

	ErrUserNotFonund = userentity.ErrUserNotFound
	ErrNotFriends    = friendentity.ErrNotFriends

	ErrNotRoomMember = roomentity.ErrMemberNotFound

	ErrConversationSequenceUpdate = errors.New("更新会话序列失败")
	ErrTooManySeqs                = errors.New("一次查询的消息序号过多")

	ErrForbidden = errors.New("无权发送消息")

	ErrUnknown = errors.New("未知错误")
)
