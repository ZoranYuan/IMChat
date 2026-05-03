package entity

import friendvo "IM_backend/internal/domain/friend/value_object"

type Friend struct {
	UserId       string
	FriendUserId string
	Remarks      string
	Status       friendvo.Status
}

func NewFriendRelation(userId, friendUserId string, remarks [2]string) []Friend {
	return []Friend{
		{
			UserId:       userId,
			FriendUserId: friendUserId,
			Remarks:      remarks[0],
			Status:       friendvo.Friend,
		},
		{
			UserId:       friendUserId,
			FriendUserId: userId,
			Remarks:      remarks[1],
			Status:       friendvo.Friend,
		},
	}
}

func (f *Friend) CanAdd(userId string) error {
	if f.UserId == userId {
		return ErrCannotAddSelf
	}
	return nil
}
