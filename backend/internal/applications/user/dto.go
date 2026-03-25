package application_user

type UserAppDTO struct {
	UserId   string `json:"userId"`
	UserName string `json:"username"`
	NickName string `json:"nickName"`
	Phone    string `json:"phone"`
	Avatar   string `json:"avatar"`
}
