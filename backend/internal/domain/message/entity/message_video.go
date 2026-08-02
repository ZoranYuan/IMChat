package entity

type MessageVideo struct {
	MessageId  string
	DurationMs int64
	Width      int
	Height     int
}

func NewMessageVideo(messageId string, durationMs int64, width, height int) *MessageVideo {
	return &MessageVideo{
		MessageId:  messageId,
		DurationMs: durationMs,
		Width:      width,
		Height:     height,
	}
}
