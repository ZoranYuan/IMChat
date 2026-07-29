package conversation

import (
	messagevo "IM_backend/internal/domain/message/value_object"
)

type PeerUserDTO struct {
	UserId   string `json:"userId"`
	UserName string `json:"userName"`
	NickName string `json:"nickName"`
	Remark   string `json:"remark"`
	Avatar   string `json:"avatar"`
}

type RoomDTO struct {
	RoomId      string `json:"roomId"`
	RoomName    string `json:"roomName"`
	Avatar      string `json:"avatar"`
	Description string `json:"description,omitempty"`
	MemberCount int    `json:"memberCount,omitempty"`
}

type LatestMessageDTO struct {
	MessageId      string          `json:"messageId"`
	ConversationId string          `json:"conversationId"`
	SenderId       string          `json:"senderId"`
	Seq            int64           `json:"seq"`
	ConvType       int8            `json:"convType"`
	CType          messagevo.CType `json:"cType"`
	Content        string          `json:"content"`
	SendTime       int64           `json:"sendTime"`
}

type ConversationItemDTO struct {
	ConversationId string            `json:"conversationId"`
	ConvType       int8              `json:"convType"`
	TargetId       string            `json:"targetId"`
	DisplayName    string            `json:"displayName"`
	Avatar         string            `json:"avatar"`
	Unread         int64             `json:"unread"`
	LastReadSeq    int64             `json:"lastReadSeq"`
	LatestSeq      int64             `json:"latestSeq"`
	LastMessage    *LatestMessageDTO `json:"lastMessage"`

	PeerUser *PeerUserDTO `json:"peerUser,omitempty"`
	Room     *RoomDTO     `json:"room,omitempty"`

	IsMuted bool `json:"isMuted"`
}
