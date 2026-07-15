package realtime

// Gateway delivers best-effort realtime events to locally connected users.
// Durable offline recovery belongs to the message store, not this interface.
type Gateway interface {
	DeliverToUser(eventType string, userID string, payload []byte) error
}
