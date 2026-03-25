package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	UserId      string     `json:"userId" gorm:"size:32;uniqueIndex;not null;comment:用户ID"`
	UserName    string     `json:"username" gorm:"size:16;uniqueIndex;not null"`
	NickName    string     `json:"nickName" gorm:"size:64"`
	Password    string     `json:"password"`
	Phone       string     `json:"phone"`
	Avatar      string     `json:"avatar"`
	Status      int        `json:"status" gorm:"default:1;comment:1正常2禁用"`
	OnLineTime  time.Time  `json:"onLineTime"`
	OffLineTime *time.Time `json:"offLineTime"`

	// 用于可以利用微信、手机号、QQ 登录
	LoginType int    `json:"loginType"`
	WxOpenID  string `json:"-" gorm:"type:varchar(64);uniqueIndex;comment:微信OpenID(预留)"`
	WxUnionID string `json:"-" gorm:"type:varchar(64);uniqueIndex;comment:微信UnionID(预留)"`
}

func (u *User) TableName() string {
	return "user"
}
