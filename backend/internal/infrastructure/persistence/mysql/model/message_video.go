package model

type MessageVideo struct {
	MessageId  string `gorm:"size:32;primaryKey" json:"messageId"`
	DurationMs int64  `gorm:"not null" json:"durationMs"`
	Width      int    `gorm:"not null" json:"width"`
	Height     int    `gorm:"not null" json:"height"`
}

func (MessageVideo) TableName() string {
	return "message_video_metadata"
}
