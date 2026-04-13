package https_friend

import (
	application_friend "IM_backend/internal/applications/friend"
)

type FriendItemRes struct {
	FriendUserId string
	FriendAvatar string
	// LastMessage    string
	// UnreadCount int
	Status      int
	DsipalyName string
}

func toFriendListRes(f []application_friend.FriendAppDTO) []FriendItemRes {
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
			DsipalyName:  displayName,
			Status:       i.Status,
		})
	}

	return res
}
