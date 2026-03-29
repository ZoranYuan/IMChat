package friend_repository_interface

import (
	friend_entity "IM_backend/internal/domain/friend/entity"

	"gorm.io/gorm"
)

type FriendRepositoryInterface interface {
	Create(domains []friend_entity.Friend) error
	FindRelation(userId, friendId string) (*friend_entity.Friend, error)
	WithTx(tx *gorm.DB) FriendRepositoryInterface
	GetUserFriendList(userId string, status []int) ([]friend_entity.Friend, error)
}
