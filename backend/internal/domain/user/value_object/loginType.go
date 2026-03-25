package user_valueobject

type LoginType int

var (
	PhoneType    LoginType = 1
	UserNameType LoginType = 2
	WxType       LoginType = 3
)
