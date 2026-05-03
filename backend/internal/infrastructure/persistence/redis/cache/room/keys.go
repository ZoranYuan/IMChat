package room

import "fmt"

func InviteKey(code string) string {
	return fmt.Sprintf("invite:%s", code)
}

// 房间ID -> 邀请码
func RoomInviteKey(roomId string) string {
	return fmt.Sprintf("room:%s:invite", roomId)
}

func RoomMembersKey(roomId string) string {
	return "room:members:" + roomId
}
