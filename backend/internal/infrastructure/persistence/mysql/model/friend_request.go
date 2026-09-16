package model

import "time"

type FriendRequest struct {
	RequestId       string `json:"requestId" gorm:"size:32;not null;primaryKey"`
	ApplicantUserId string `json:"applicantUserId" gorm:"size:32;not null;uniqueIndex:uk_friend_request_pair,priority:1"`
	PeerUserId      string `json:"peerUserId" gorm:"size:32;not null;uniqueIndex:uk_friend_request_pair,priority:2;index:idx_friend_request_peer_updated,priority:1"`
	Status          int    `json:"status" gorm:"tinyInt;default:0;comment:1-待处理，2-已同意，3-已拒绝"`
	Message         string `gorm:"size:128"`
	ApplyTime       int64
	CreatedAt       time.Time
	UpdatedAt       time.Time `gorm:"index:idx_friend_request_peer_updated,priority:2"`
}

func (FriendRequest) TableName() string {
	return "friend_requests"
}
