package conversation

import (
	messagevo "IM_backend/internal/domain/message/value_object"
)

type PeerUserDTO struct {
	UserID    string
	Username  string
	Nickname  string
	Remark    string
	AvatarURL string
}

type RoomDTO struct {
	RoomID      string
	RoomName    string
	AvatarURL   string
	Description string
	MemberCount int
}

type LatestMessageDTO struct {
	MessageID      string
	ConversationID string
	SenderID       string
	Seq            int64
	Type           messagevo.CType
	Content        string
	SendTime       int64
}

type ConversationItemDTO struct {
	ConversationID   string
	ConversationType int8
	TargetID         string
	DisplayName      string
	AvatarURL        string
	Unread           int64
	LastMessage      *LatestMessageDTO

	PeerUser *PeerUserDTO
	Room     *RoomDTO

	IsMuted bool
}
