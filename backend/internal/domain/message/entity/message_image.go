package entity

type MessageImage struct {
	MessageId string
	Width     int
	Height    int
	MimeType  string
}

func NewMessageImage(messageId, mimeType string, width, height int) *MessageImage {
	return &MessageImage{
		MessageId: messageId,
		Width:     width,
		Height:    height,
		MimeType:  mimeType,
	}
}
