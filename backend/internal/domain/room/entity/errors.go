package room_entity

import "errors"

var (
	ErrDuplicateCreate   = errors.New("重复创建")
	ErrDuplicateJoin     = errors.New("重复加入")
	ErrDuplicateLeft     = errors.New("重复退出")
	ErrUnavaiableCode    = errors.New("邀请码失效")
	ErrRoomNameIsNotNull = errors.New("房间名称不能为空")
	ErrRoomNotFound      = errors.New("房间不存在")
	ErrRecordNotFound    = errors.New("未查询到记录")
	ErrNoPermission      = errors.New("没有权限")
	ErrVersionConflict   = errors.New("版本不一致")
)
