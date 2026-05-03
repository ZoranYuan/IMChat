package friend_cache_interface

import "context"

type FriendCache interface {
	IsFriend(ctx context.Context, userId, friendUserId string) (bool, error)
	AddFriend(ctx context.Context, userId, friendUserId string) error
	GetFriends(ctx context.Context, userId string) ([]string, error)
	RemoveFriend(ctx context.Context, userId, friendUserId string) error
	DeleteUsersFriends(ctx context.Context, userId []string) error
}
