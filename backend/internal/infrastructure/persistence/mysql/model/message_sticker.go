package model

type MessageSticker struct {
	MessageId string `gorm:"size:32;primaryKey" json:"messageId"`
	StickerId string `gorm:"size:32;not null;index" json:"stickerId"`
	PackId    string `gorm:"size:32;index" json:"packId,omitempty"`
	Width     int    `gorm:"not null" json:"width"`
	Height    int    `gorm:"not null" json:"height"`
}
