package realtime

type Delivery interface {
	DeliverToUser(eventType, userID string, payload []byte) error
}
