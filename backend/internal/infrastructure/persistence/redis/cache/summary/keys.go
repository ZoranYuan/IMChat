package summary

import cachekey "IM_backend/internal/infrastructure/persistence/redis/cache/key"

func ActiveRoomUnreadSnapshotKey(userID, roomID string) string {
	return cachekey.Build("summary", "active", "room", "unread", userID, roomID)
}

func SummaryScopeRunKey(userID, roomID string) string {
	return cachekey.Build("summary", "scope", "run", userID, roomID)
}
