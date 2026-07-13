package model

type MessageVideo struct {
	MessageId   string `gorm:"size:32;primaryKey" json:"messageId"`
	FileId      string `gorm:"size:32;not null;index" json:"fileId"`
	CoverFileId string `gorm:"size:32;index" json:"coverFileId,omitempty"`
	DurationMs  int64  `gorm:"not null" json:"durationMs"`
	Width       int    `gorm:"not null" json:"width"`
	Height      int    `gorm:"not null" json:"height"`
	URL         string `gorm:"size:512" json:"url"`
}
