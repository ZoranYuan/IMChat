package application_user

type UserAppDTO struct {
	UserId    string `json:"userId"`
	UserName  string `json:"username"`
	NickName  string `json:"nickName"`
	Phone     string `json:"phone"`
	Avatar    string `json:"avatar"`
	WxOpenID  string `json:"-" gorm:"type:varchar(64);index;comment:微信OpenID(预留)"`
	WxUnionID string `json:"-" gorm:"type:varchar(64);index;comment:微信UnionID(预留)"`
}
