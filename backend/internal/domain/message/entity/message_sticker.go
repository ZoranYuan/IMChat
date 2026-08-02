package entity

type MessageSticker struct {
	MessageId string
	StickerId string
	PackId    string
	Width     int
	Height    int
}

func NewMessageSticker(messageId, stickerId, packId string, width, height int) *MessageSticker {
	return &MessageSticker{
		MessageId: messageId,
		StickerId: stickerId,
		PackId:    packId,
		Width:     width,
		Height:    height,
	}
}
