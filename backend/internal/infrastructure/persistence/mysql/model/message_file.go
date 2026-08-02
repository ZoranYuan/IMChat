package model

type MessageFile struct {
	MessageId    string `gorm:"size:32;primaryKey" json:"messageId"`
	DownloadName string `gorm:"size:255" json:"downloadName,omitempty"`
}

func (MessageFile) TableName() string {
	return "message_file_metadata"
}
