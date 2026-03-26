package friend

type FriendRequestReq struct {
	RequestId string `json:"requestId; binding:requried"`
	Message   string `json:"message; binding:requried"` // 可选留言
}
