package friendrequest

type FriendRequestReq struct {
	ToUserId string `json:"toUserId" binding:"required"`
	Message  string `json:"message"` // 可选留言
}

type ActionType int

const (
	ActionAccept ActionType = 1
	ActionReject ActionType = 2
)

type FriendRequestRes struct {
	RequestId       string `json:"requestId"`
	FromUserId      string `json:"fromUserId"`
	FromUsername    string `json:"fromUsername"`
	FromDisplayName string `json:"fromDisplayName"`
	ToUserId        string `json:"toUserId"`
	Message         string `json:"message"`
	Status          int    `json:"status"`
	ApplyTime       int64  `json:"applyTime"`
}

type OperateRequestReq struct {
	RequestId  string     `json:"requestId" binding:"required"`
	FromUserId string     `json:"fromUserId" binding:"required"`
	Action     ActionType `json:"action" binding:"required"`
}
