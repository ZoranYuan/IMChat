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
	WxOpenID  *string          `json:"-" gorm:"type:varchar(64);index;comment:微信OpenID(预留)"`
	WxUnionID *string          `json:"-" gorm:"type:varchar(64);index;comment:微信UnionID(预留)"`
}

func RegisterWithPhone(phone uservo.Phone, password uservo.Password) (*User, error) {
	if !phone.Validate() {
		return nil, ErrInvalidPhoneNumber
	}

	// hash 加密
	hp, err := password.GenPasswordHash()

	// 生成 UserId

	if err != nil {
		return nil, err
	}

	var newUser = User{
		Phone:       phone,
		LoginType:   uservo.PhoneType,
		Status:      uservo.StatusActivate,
		Password:    hp,
		UserName:    string(phone),
		OnLineTime:  time.Now(),
		OffLineTime: nil,
	}

	return &newUser, nil
}

func LoginWithPhone(phone uservo.Phone, password uservo.Password, hPassword string) error {
	if !phone.Validate() {
		return ErrInvalidPhoneNumber
	}

	// 解密
	if isCheck := password.VerifyPasswordHash([]byte(hPassword)); !isCheck {
		return ErrInvalidPhoneNumber
	}

	return nil
}
