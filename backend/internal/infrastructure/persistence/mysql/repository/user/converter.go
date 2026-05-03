package user

import (
	userentity "IM_backend/internal/domain/user/entity"
	uservo "IM_backend/internal/domain/user/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func toDomain(m model.User) userentity.User {
	return userentity.User{
		UserId:      m.UserId,
		UserName:    m.UserName,
		NickName:    m.NickName,
		Password:    uservo.Password(m.Password),
		Phone:       uservo.Phone(m.Phone),
		Avatar:      m.Avatar,
		Status:      uservo.Status(m.Status),
		OnLineTime:  m.OnLineTime,
		OffLineTime: m.OffLineTime,

		// 用于可以利用微信、手机号、QQ 登录
		LoginType: uservo.LoginType(m.LoginType),
		WxOpenID:  m.WxOpenID,
		WxUnionID: m.WxUnionID,
	}
}
