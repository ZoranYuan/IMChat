package friend

import (
	friendapp "IM_backend/internal/application/friend"
)

type FriendItemRes struct {
	FriendUserId string `json:"friendUserId"`
	FriendAvatar string `json:"friendAvatar"`
	Status       int    `json:"status"`
	DisplayName  string `json:"displayName"`
}

func toFriendListRes(f []friendapp.FriendAppDTO) []FriendItemRes {
	var res = make([]FriendItemRes, 0, len(f))

	for _, i := range f {
		var displayName string
		if i.Remarks != "" {
			displayName = i.Remarks
		} else if i.FriendNickName != "" {
			displayName = i.FriendNickName
		} else {
			displayName = i.FriendUserName
		}

		res = append(res, FriendItemRes{
			FriendUserId: i.FriendUserId,
			FriendAvatar: i.FriendAvatar,
			DisplayName:  displayName,
			Status:       i.Status,
		})
	}

	return res
}
