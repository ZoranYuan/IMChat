package model

import (
	"gorm.io/gorm"
)

type RoomUser struct {
	UserId    string `json:"userId" gorm:"size:32;primaryKey;not null;comment:用户ID"`
	RoomId    string `json:"roomId" gorm:"size:32;index;primaryKey;not null"`
	Role      int    `gorm:"tinyInt;default:0;comment:0普通用户,1管理员,2房主"`
	Status    int    `json:"status" gorm:"comment: 成员状态"`
	MuteUtil  *int64 // 禁言到什么时候（时间戳）
	JoinTime  int64
	LeaveTime *int64
	Version   int64
	gorm.Model
}

func (u *RoomUser) TableName() string {
	return "room_user"
}
