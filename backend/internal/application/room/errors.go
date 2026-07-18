package room

import (
	roomentity "IM_backend/internal/domain/room/entity"
	"errors"
)

var (
	ErrRoomNameRequired      = roomentity.ErrRoomNameRequired
	ErrRoomNotFound          = roomentity.ErrRoomNotFound
	ErrUnknown               = errors.New("未知错误")
	ErrNotRoomMember         = roomentity.ErrMemberNotFound
	ErrRoomUnavailable       = errors.New("房间当前不可用")
	ErrInviteCodeExpired     = roomentity.ErrInviteCodeExpired
	ErrConcurrentUpdate      = roomentity.ErrVersionConflict
	ErrPermissionDenied      = roomentity.ErrPermissionDenied
	ErrInviteCodeUnavailable = errors.New("邀请码服务暂时不可用")
)
