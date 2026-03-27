package https_friend_request

import "time"

type FriendRequestReq struct {
	ToUserId string `json:"toUserId" binding:"required"`
	Message  string `json:"message" binding:"required"` // 可选留言
}

type FriendRequestRes struct {
	RequestId string    `json:"requestId"`
	ToUserId  string    `json:"toUserId"`
	Message   string    `json:"message"`
	Status    int       `json:"status"`
	ApplyTime time.Time `json:"applyTime"`
}

type OperateRequestReq struct {
	RequestId string `json:"requestId" binding:"required"`
	IsAccept  bool   `json:"isAccept" binding:"required"` // 可选留言
}
