package room

type CreateRoomReq struct {
	Description string `json:"description"`
	RoomName    string `json:"roomName" binding:"required"`
	Avatar      string `json:"avatar"`
}

type CreateRoomRes struct {
	RoomId      string `json:"roomId"`
	Description string `json:"description"`
	RoomName    string `json:"roomName"`
	Avatar      string `json:"avatar"`
	MemberCount int    `json:"memberCount"`
	InviteCode  string `json:"inviteCode"`
}

type JoinRoomReq struct {
	InviteCode string `json:"inviteCode" binding:"required"`
}

type JoinRoomRes struct {
	RoomId      string `json:"roomId"`
	Description string `json:"description"`
	RoomName    string `json:"roomName"`
	Avatar      string `json:"avatar"`
	MemberCount int    `json:"memberCount"`
	Role        int    `json:"role"`
	JoinTime    int64  `json:"joinTime"`
}
