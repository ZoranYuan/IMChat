package friend

import "context"

type FriendCache interface {
	IsFriend(ctx context.Context, userId, friendUserId string) (bool, error)
	AddFriend(ctx context.Context, userId, friendUserId string) error
	GetFriends(ctx context.Context, userId string) ([]string, error)
	RemoveFriend(ctx context.Context, userId, friendUserId string) error
	DeleteUserFriends(ctx context.Context, userIds []string) error
}
