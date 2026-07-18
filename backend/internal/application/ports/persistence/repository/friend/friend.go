package friend

import (
	friendentity "IM_backend/internal/domain/friend/entity"
)

type FriendRepository interface {
	Create(domains []friendentity.Friend) error
	FindRelation(userId, friendId string) (*friendentity.Friend, error)
	WithTx(tx any) FriendRepository
	GetUserFriendList(userId string, status int) ([]friendentity.Friend, error)
}
