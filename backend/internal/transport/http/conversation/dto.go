package conversation

import (
	conversationapp "IM_backend/internal/application/conversation"
)

type PeerUserResponse struct {
	UserID    string `json:"userId"`
	Username  string `json:"userName"`
	Nickname  string `json:"nickName"`
	Remark    string `json:"remark"`
	AvatarURL string `json:"avatar"`
}

type RoomResponse struct {
	RoomID      string `json:"roomId"`
	RoomName    string `json:"roomName"`
	AvatarURL   string `json:"avatar"`
	Description string `json:"description,omitempty"`
	MemberCount int    `json:"memberCount,omitempty"`
}

type LatestMessageResponse struct {
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
	SenderID       string `json:"senderId"`
	Seq            int64  `json:"seq"`
	Type           int8   `json:"cType"`
	Content        string `json:"content"`
	SendTime       int64  `json:"sendTime"`
}

type ConversationItemResponse struct {
	ConversationID   string                 `json:"conversationId"`
	ConversationType int8                   `json:"convType"`
	TargetID         string                 `json:"targetId"`
	DisplayName      string                 `json:"displayName"`
	AvatarURL        string                 `json:"avatar"`
	Unread           int64                  `json:"unread"`
	LastMessage      *LatestMessageResponse `json:"lastMessage,omitempty"`
	PeerUser         *PeerUserResponse      `json:"peerUser,omitempty"`
	Room             *RoomResponse          `json:"room,omitempty"`
	IsMuted          bool                   `json:"isMuted"`
}

func toConversationResponses(items []conversationapp.ConversationItemDTO) []ConversationItemResponse {
	res := make([]ConversationItemResponse, 0, len(items))
	for _, item := range items {
		resItem := ConversationItemResponse{
			ConversationID:   item.ConversationID,
			ConversationType: item.ConversationType,
			TargetID:         item.TargetID,
			DisplayName:      item.DisplayName,
			AvatarURL:        item.AvatarURL,
			Unread:           item.Unread,
			IsMuted:          item.IsMuted,
		}

		if item.LastMessage != nil {
			resItem.LastMessage = &LatestMessageResponse{
				MessageID:      item.LastMessage.MessageID,
				ConversationID: item.LastMessage.ConversationID,
				SenderID:       item.LastMessage.SenderID,
				Seq:            item.LastMessage.Seq,
				Type:           int8(item.LastMessage.Type),
				Content:        item.LastMessage.Content,
				SendTime:       item.LastMessage.SendTime,
			}
		}
		if item.PeerUser != nil {
			resItem.PeerUser = &PeerUserResponse{
				UserID:    item.PeerUser.UserID,
				Username:  item.PeerUser.Username,
				Nickname:  item.PeerUser.Nickname,
				Remark:    item.PeerUser.Remark,
				AvatarURL: item.PeerUser.AvatarURL,
			}
		}
		if item.Room != nil {
			resItem.Room = &RoomResponse{
				RoomID:      item.Room.RoomID,
				RoomName:    item.Room.RoomName,
				AvatarURL:   item.Room.AvatarURL,
				Description: item.Room.Description,
				MemberCount: item.Room.MemberCount,
			}
		}

		res = append(res, resItem)
	}

	return res
}
