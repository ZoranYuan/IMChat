package entity

type MessageSticker struct {
	MessageId string
	StickerId string
	PackId    string
	URL       string
	Width     int
	Height    int
}

func NewMessageSticker(messageId, stickerId, packId, url string, width, height int) *MessageSticker {
	return &MessageSticker{
		MessageId: messageId,
		StickerId: stickerId,
		PackId:    packId,
		URL:       url,
		Width:     width,
		Height:    height,
	}
}
