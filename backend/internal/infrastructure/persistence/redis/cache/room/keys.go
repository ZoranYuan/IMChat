package room

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func RoomProfile(roomId string) string {
	return "im:room:" + roomId + ":profile"
}

func InviteKey(code string) string {
	return cachekey.RoomInviteCode(code)
}

func InviteKeyPrefix() string {
	return cachekey.RoomInviteCode("") + ":"
}

// 房间ID -> 邀请码
func RoomInviteKey(roomId string) string {
	return cachekey.RoomInvite(roomId)
}

func RoomMembersKey(roomId string) string {
	return cachekey.RoomMembers(roomId)
}

func RoomMemberKey(roomID, userID string) string {
	return cachekey.RoomMember(roomID, userID)
}
