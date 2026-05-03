package user_repository

import (
	user_entity "IM_backend/internal/domain/user/entity"
	user_valueobject "IM_backend/internal/domain/user/value_object"
	"IM_backend/internal/infrastructure/persistence/mysql/model"
)

func toDomain(m model.User) user_entity.User {
	return user_entity.User{
		UserId:      m.UserId,
		UserName:    m.UserName,
		NickName:    m.NickName,
		Password:    user_valueobject.Password(m.Password),
		Phone:       user_valueobject.Phone(m.Phone),
		Avatar:      m.Avatar,
		Status:      user_valueobject.Status(m.Status),
		OnLineTime:  m.OnLineTime,
		OffLineTime: m.OffLineTime,

		// 用于可以利用微信、手机号、QQ 登录
		LoginType: user_valueobject.LoginType(m.LoginType),
		WxOpenID:  m.WxOpenID,
		WxUnionID: m.WxUnionID,
	}
}
