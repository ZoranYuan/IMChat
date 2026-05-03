package model

import "time"

type FriendRequest struct {
	RequestId  string `json:"requestId" gorm:"size:32;not null;primaryKey"`
	FromUserId string `json:"fromUserId" gorm:"size:32;not null;index:idx_from_to,priority:1"`
	ToUserId   string `json:"toUserId" gorm:"size:32;not null;index:idx_from_to,priority:2"`
	Status     int    `json:"status" gorm:"tinyInt;default:0;comment:1-待处理，2-已同意，3-已拒绝"`
	Message    string `gorm:"size:128"`
	ApplyTime  int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (FriendRequest) TableName() string {
	return "friend_request"
}
