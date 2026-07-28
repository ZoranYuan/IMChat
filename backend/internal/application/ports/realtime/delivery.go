package realtime

type Delivery interface {
	DeliverToUser(eventType, userID string, payload []byte) error
}

type RoomDelivery interface {
	Delivery
	DeliverToOnlineRoomMembers(eventType, roomID string, payload []byte, excludeUserID string) error
}

type RoomPresence interface {
	BindUserToRoom(userID, roomID string)
	UnbindUserFromRoom(userID, roomID string)
}
