package entity

type MessageImage struct {
	MessageId   string
	FileId      string
	ThumbFileId string
	Width       int
	Height      int
	MimeType    string
	Size        int64
	URL         string
}

func NewMessageImage(messageId, fileId, thumbFileId, mimeType, url string, width, height int, size int64) *MessageImage {
	return &MessageImage{
		MessageId:   messageId,
		FileId:      fileId,
		ThumbFileId: thumbFileId,
		Width:       width,
		Height:      height,
		MimeType:    mimeType,
		Size:        size,
		URL:         url,
	}
}
