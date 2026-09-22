package model

type SummaryRun struct {
	SummaryRunID    string `gorm:"size:64;primaryKey"`
	RoomID          string `gorm:"size:64;not null;index:idx_summary_run_scope"`
	UserID          string `gorm:"size:64;not null;index:idx_summary_run_scope"`
	Status          string `gorm:"size:32;not null;index"`
	FromSeq         int64  `gorm:"not null;default:0"`
	ToSeq           int64  `gorm:"not null;default:0"`
	LatestRequestID string `gorm:"size:64;not null;default:''"`
	ResponsePayload string `gorm:"type:json"`
	CreatedAt       int64  `gorm:"not null"`
	UpdatedAt       int64  `gorm:"not null"`
	FinishedAt      *int64
}
