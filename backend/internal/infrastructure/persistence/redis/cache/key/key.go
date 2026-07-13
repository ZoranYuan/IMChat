package key

import "strings"

const Prefix = "im"

func Build(parts ...string) string {
	items := make([]string, 0, len(parts)+1)
	items = append(items, Prefix)
	for _, part := range parts {
		part = strings.Trim(part, ":")
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return strings.Join(items, ":")
}

func AuthAccessToken(token string) string {
	return Build("auth", "token", "access", token)
}

func AuthAccessUser(userId string) string {
	return Build("auth", "user", userId, "access")
}

func AuthRefreshUser(userId string) string {
	return Build("auth", "user", userId, "refresh")
}

func AuthRefreshToken(token string) string {
	return Build("auth", "token", "refresh", token)
}

func ConversationMembers(conversationId string) string {
	return Build("conversation", conversationId, "members")
}

func ConversationSeq(conversationId string) string {
	return Build("conversation", conversationId, "seq")
}

func ConversationMembersVersion(conversationId string) string {
	return Build("conversation", conversationId, "members", "version")
}

func FileMeta(fileId string) string {
	return Build("file", fileId, "meta")
}

func UserInfo(userId string) string {
	return Build("user", userId, "info")
}

func UserFriends(userId string) string {
	return Build("user", userId, "friends")
}

func RoomInviteCode(code string) string {
	return Build("room", "invite", code)
}

func RoomInvite(roomId string) string {
	return Build("room", roomId, "invite")
}

func RoomMembers(roomId string) string {
	return Build("room", roomId, "members")
}

func MessageDedup(clientMsgId string) string {
	return Build("msg", "dedup", clientMsgId)
}
