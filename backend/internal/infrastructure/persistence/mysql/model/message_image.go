package model

type MessageImage struct {
	MessageId string `gorm:"size:32;primaryKey" json:"messageId"`
	Width     int    `gorm:"not null" json:"width"`
	Height    int    `gorm:"not null" json:"height"`
	MimeType  string `gorm:"size:128" json:"mimeType"`
	Size      int64  `gorm:"not null" json:"size"`
}

func (MessageImage) TableName() string {
	return "message_image_metadata"
}
