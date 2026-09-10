package entity

import (
	uservo "IM_backend/internal/domain/user/value_object"
	"time"
)

type User struct {
	UserId      string          `json:"userId"`
	UserName    string          `json:"username"`
	NickName    string          `json:"nickName"`
	Password    uservo.Password `json:"password"`
	Phone       uservo.Phone    `json:"phone"`
	Avatar      string          `json:"avatar"`
	Status      uservo.Status   `json:"status"`
	OnLineTime  time.Time       `json:"onLineTime"`
	OffLineTime *time.Time      `json:"offLineTime"`

	// 用于可以利用微信、手机号、QQ 登录
	LoginType uservo.LoginType `json:"loginType"`
	WxOpenID  *string          `json:"-"`
	WxUnionID *string          `json:"-"`
}

func RegisterWithPhone(phone uservo.Phone, password uservo.Password) (*User, error) {
	if !phone.Validate() {
		return nil, ErrInvalidPhoneNumber
	}

	var newUser = User{
		Phone:       phone,
		LoginType:   uservo.PhoneType,
		Status:      uservo.StatusActivate,
		Password:    password,
		UserName:    string(phone),
		NickName:    string(phone),
		OnLineTime:  time.Now(),
		OffLineTime: nil,
	}

	return &newUser, nil
}
