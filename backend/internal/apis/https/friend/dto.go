package https_friend

import application_friend "IM_backend/internal/applications/friend"

type FriendListRes struct {
	FriendUserId string
	FriendAvatar string
	// LastMessage    string
	// UnreadCount int
	Remarks string
}

func toRes(f []application_friend.FriendAppDTO) []FriendListRes {
	var res = make([]FriendListRes, len(f))

	for _, i := range f {
		res = append(res, FriendListRes{
			FriendUserId: i.FriendUserId,
			FriendAvatar: i.FriendAvatar,
			Remarks:      i.Remarks,
		})
	}

	return res
}
