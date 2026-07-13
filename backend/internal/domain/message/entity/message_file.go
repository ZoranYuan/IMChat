package entity

type MessageFile struct {
	MessageId    string
	FileId       string
	FileName     string
	DownloadName string
	MimeType     string
	Size         int64
	URL          string
}

func NewMessageFile(messageId, fileId, fileName, downloadName, mimeType, url string, size int64) *MessageFile {
	return &MessageFile{
		MessageId:    messageId,
		FileId:       fileId,
		FileName:     fileName,
		DownloadName: downloadName,
		MimeType:     mimeType,
		Size:         size,
		URL:          url,
	}
}
