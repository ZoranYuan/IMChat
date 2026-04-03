package model

import "time"

type Friend struct {
	UserId       string `json:"userId" gorm:"size:32;not null;primaryKey;comment:用户ID"`
	FriendUserId string `json:"friendUserId" gorm:"size:32;not null;primaryKey;comment:好友 Id"`
	Remarks      string `json:"remarks" gorm:"size:64;comment:好友备注"`
	Status       int    `json:"status" gorm:"tinInt;comment:好友状态(0-好友，1-拉黑对方，2-被对方拉黑，3-删除, 4-待同意，5-已申请)"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *Friend) TableName() string {
	return "friend"
}
