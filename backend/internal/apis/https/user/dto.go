package https_user

type UserRegisterReq struct {
	Phone             string `json:"phone"`
	Password          string `json:"password"`
	ReconfirmPassword string `json:"reconfirmPassword"`
	LoginType         int    `json:"loginType" binding:"required"`
}

type UserRegisterRes struct {
	Token     string `json:"token"`
	UserId    string `json:"userId"`
	UserName  string `json:"username"`
	NickName  string `json:"nickName"`
	Phone     string `json:"phone"`
	Avatar    string `json:"avatar"`
	WxOpenID  string `json:"-" gorm:"type:varchar(64);index;comment:微信OpenID(预留)"`
	WxUnionID string `json:"-" gorm:"type:varchar(64);index;comment:微信UnionID(预留)"`
}

type UserLoginReq struct {
	LoginType int    `json:"loginType" binding:"required"`
	Phone     string `json:"phone"`
	Password  string `json:"password"`
	UserName  string `json:"userName"`
}

type UserLoginRes struct {
	Token     string `json:"token"`
	UserId    string `json:"userId"`
	UserName  string `json:"username"`
	NickName  string `json:"nickName"`
	Phone     string `json:"phone"`
	Avatar    string `json:"avatar"`
	WxOpenID  string `json:"-" gorm:"type:varchar(64);index;comment:微信OpenID(预留)"`
	WxUnionID string `json:"-" gorm:"type:varchar(64);index;comment:微信UnionID(预留)"`
}

type UserLogoutReq struct {
	UserId string `json:"userId" binding:"required"`
}
