package entity

type MessageVideo struct {
	MessageId   string
	FileId      string
	CoverFileId string
	DurationMs  int64
	Width       int
	Height      int
	URL         string
}

func NewMessageVideo(messageId, fileId, coverFileId, url string, durationMs int64, width, height int) *MessageVideo {
	return &MessageVideo{
		MessageId:   messageId,
		FileId:      fileId,
		CoverFileId: coverFileId,
		DurationMs:  durationMs,
		Width:       width,
		Height:      height,
		URL:         url,
	}
}
