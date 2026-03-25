package model

import "gorm.io/gorm"

type RoomUser struct {
	gorm.Model
	UserId   string `json:"userId" gorm:"size:32;uniqueIndex;not null;comment:用户ID"`
	RoomId   string `json:"roomId" gorm:"size:32;not null"`
	Role     int    `gorm:"tinyInt;default:0;comment:0普通用户,1管理员,2房主"`
	RoomName string `json:"roomName" gorm:"size:10;comment:群昵称"`
}

func (u *RoomUser) TableName() string {
	return "room_user"
}
