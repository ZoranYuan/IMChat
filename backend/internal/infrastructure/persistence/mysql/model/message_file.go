package model

type MessageFile struct {
	MessageId    string `gorm:"size:32;primaryKey" json:"messageId"`
	FileId       string `gorm:"size:32;not null;index" json:"fileId"`
	FileName     string `gorm:"size:255;not null" json:"fileName"`
	DownloadName string `gorm:"size:255" json:"downloadName,omitempty"`
	MimeType     string `gorm:"size:128" json:"mimeType"`
	Size         int64  `gorm:"not null" json:"size"`
	URL          string `gorm:"size:512" json:"url"`
}
