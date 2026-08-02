package entity

type MessageFile struct {
	MessageId    string
	DownloadName string
}

func NewMessageFile(messageId, downloadName string) *MessageFile {
	return &MessageFile{
		MessageId:    messageId,
		DownloadName: downloadName,
	}
}
