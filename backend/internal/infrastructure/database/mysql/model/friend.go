package model

import "gorm.io/gorm"

type Friend struct {
	gorm.Model
	UserId       string `json:"userId" gorm:"size:32;not null;index:idx_user_friend,unique;comment:用户ID"`
	FriendUserId string `json:"friendUserId" gorm:"size:32;not null;index:idx_user_friend,unique;comment:好友 Id"`
	UserName     string `json:"userName" gorm:"size:64;not null;comment:好友用户名"`
	NickName     string `json:"nickName" gorm:"size:64;comment:好友昵称"`
	Remarks      string `json:"remarks" gorm:"size:64;comment:好友备注"`
	Status       int    `json:"status" gorm:"tinInt;comment:好友状态(0-好友，1-拉黑对方，2-被对方拉黑，3-删除, 4-待同意，5-已申请)"`
}

func (u *Friend) TableName() string {
	return "friend"
}
