package application_room

import "errors"

var (
	ErrRoomNameIsNotNull = errors.New("房间名称不能为空")
	ErrRoomNotFound      = errors.New("房间不存在")
	ErrUnknownError      = errors.New("网络错误")
	ErrNotAvaiableRoom   = errors.New("房间目前不可用")
)
