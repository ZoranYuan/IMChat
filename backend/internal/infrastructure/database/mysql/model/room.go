package model

import "gorm.io/gorm"

type Room struct {
	RoomId      string `json:"userId" gorm:"size:32;primaryKey;not null"`
	OwnerUserId string `json:"ownerUserId" gorm:"size:32;not null"`
	Description string `json:"description" gorm:"comment:群介绍"`
	RoomName    string `json:"roomName" gorm:"size:10;index;comment:群昵称"`
	Status      int    `json:"status" gorm:"tinyInt;index:idx_room_status;comment:1为正常，0为暂时不可用"`
	Avatar      string `json:"avatar" gorm:"size:255;comment:房间头像URL"`
	MemberCount int    `json:"memberCount" gorm:"type:int;default:0;comment:房间人数"`
	MaxMembers  int    `json:"maxUsers" gorm:"default:100;comment:房间最大人数"`
	gorm.Model
}

func (u *Room) TableName() string {
	return "room"
}
