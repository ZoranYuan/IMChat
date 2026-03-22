package user_entity

import (
	user_valueobject "IM_backend/internal/domain/user/value_object"
	"time"
)

type User struct {
	UserId      string                    `json:"userId"`
	UserName    string                    `json:"username"`
	NickName    string                    `json:"nickName"`
	Password    user_valueobject.Password `json:"password"`
	Phone       user_valueobject.Phone    `json:"phone"`
	Avatar      string                    `json:"avatar"`
	Status      user_valueobject.Status   `json:"status"`
	OnLineTime  time.Time                 `json:"onLineTime"`
	OffLineTime time.Time                 `json:"offLineTime"`

	// 用于可以利用微信、手机号、QQ 登录
	LoginType user_valueobject.Status `json:"loginType"`
	WxOpenID  string                  `json:"-" gorm:"type:varchar(64);index;comment:微信OpenID(预留)"`
	WxUnionID string                  `json:"-" gorm:"type:varchar(64);index;comment:微信UnionID(预留)"`
}
