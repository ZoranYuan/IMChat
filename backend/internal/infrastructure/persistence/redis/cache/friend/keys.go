package friend

import "fmt"

const (
	FriendSetKeyPrefix = "friend:set:" // user -> set(friendUserId)
)

func FriendSetKey(userId string) string {
	return fmt.Sprintf("%s%s", FriendSetKeyPrefix, userId)
}
