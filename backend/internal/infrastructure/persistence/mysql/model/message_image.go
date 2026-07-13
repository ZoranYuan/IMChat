package model

type MessageImage struct {
	MessageId   string `gorm:"size:32;primaryKey" json:"messageId"`
	FileId      string `gorm:"size:32;not null;index" json:"fileId"`
	ThumbFileId string `gorm:"size:32;index" json:"thumbFileId,omitempty"`
	Width       int    `gorm:"not null" json:"width"`
	Height      int    `gorm:"not null" json:"height"`
	MimeType    string `gorm:"size:128" json:"mimeType"`
	Size        int64  `gorm:"not null" json:"size"`
	URL         string `gorm:"size:512" json:"url"`
}
