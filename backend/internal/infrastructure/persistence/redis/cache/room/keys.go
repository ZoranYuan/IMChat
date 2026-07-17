package room

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func InviteKey(code string) string {
	return cachekey.RoomInviteCode(code)
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
