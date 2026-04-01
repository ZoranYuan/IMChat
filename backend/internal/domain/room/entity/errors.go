package room_entity

import "errors"

var (
	ErrRoomNameIsNotNull = errors.New("房间名称不能为空")
	ErrRoomNotFound      = errors.New("房间不存在")
)
