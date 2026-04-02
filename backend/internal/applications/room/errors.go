package application_room

import "errors"

var (
	ErrRoomNameIsNotNull = errors.New("房间名称不能为空")
	ErrRoomNotFound      = errors.New("房间不存在")
	ErrUnknownError      = errors.New("网络错误")
	ErrNotInRoom         = errors.New("不在房间内")
	ErrUselessCode       = errors.New("无效的验证码")
	ErrNotAvaiableRoom   = errors.New("房间目前不可用")
	ErrUnavaiableCode    = errors.New("邀请码失效")
	ErrConcurrentUpdate  = errors.New("请勿重复操作")
	ErrNoPermission      = errors.New("没有权限")
)
