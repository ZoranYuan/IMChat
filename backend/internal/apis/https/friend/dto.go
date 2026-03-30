package https_friend

import (
	application_friend "IM_backend/internal/applications/friend"
)

type FriendListRes struct {
	FriendUserId string
	FriendAvatar string
	// LastMessage    string
	// UnreadCount int
	Status      int
	DsipalyName string
}

func toRes(f []application_friend.FriendAppDTO) []FriendListRes {
	var res = make([]FriendListRes, 0, len(f))

	for _, i := range f {
		var displayName string
		if i.Remarks != "" {
			displayName = i.Remarks
		} else if i.FriendNickName != "" {
			displayName = i.FriendNickName
		} else {
			displayName = i.FriendUserName
		}

		res = append(res, FriendListRes{
			FriendUserId: i.FriendUserId,
			FriendAvatar: i.FriendAvatar,
			DsipalyName:  displayName,
			Status:       i.Status,
		})
	}

	return res
}
