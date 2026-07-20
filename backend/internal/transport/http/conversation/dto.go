package conversation

import (
	conversationapp "IM_backend/internal/application/conversation"
)

type PeerUserRes struct {
	UserId   string `json:"userId"`
	UserName string `json:"userName"`
	NickName string `json:"nickName"`
	Remark   string `json:"remark"`
	Avatar   string `json:"avatar"`
}

type RoomRes struct {
	RoomId      string `json:"roomId"`
	RoomName    string `json:"roomName"`
	Avatar      string `json:"avatar"`
	Description string `json:"description,omitempty"`
	MemberCount int    `json:"memberCount,omitempty"`
}

type LatestMessageRes struct {
	MessageId      string `json:"messageId"`
	ConversationId string `json:"conversationId"`
	SenderId       string `json:"senderId"`
	Seq            int64  `json:"seq"`
	ConvType       int8   `json:"convType"`
	CType          int8   `json:"cType"`
	Content        string `json:"content"`
	SendTime       int64  `json:"sendTime"`
}

type ConversationItemRes struct {
	ConversationId string            `json:"conversationId"`
	ConvType       int8              `json:"convType"`
	TargetId       string            `json:"targetId"`
	DisplayName    string            `json:"displayName"`
	Avatar         string            `json:"avatar"`
	Unread         int64             `json:"unread"`
	LastMessage    *LatestMessageRes `json:"lastMessage,omitempty"`
	PeerUser       *PeerUserRes      `json:"peerUser,omitempty"`
	Room           *RoomRes          `json:"room,omitempty"`
	IsMuted        bool              `json:"isMuted"`
}

func toUserConversationsRes(items []conversationapp.ConversationItemDTO) []ConversationItemRes {
	res := make([]ConversationItemRes, 0, len(items))
	for _, item := range items {
		resItem := ConversationItemRes{
			ConversationId: item.ConversationId,
			ConvType:       item.ConvType,
			TargetId:       item.TargetId,
			DisplayName:    item.DisplayName,
			Avatar:         item.Avatar,
			Unread:         item.Unread,
			IsMuted:        item.IsMuted,
		}

		if item.LastMessage != nil {
			resItem.LastMessage = &LatestMessageRes{
				MessageId:      item.LastMessage.MessageId,
				ConversationId: item.LastMessage.ConversationId,
				SenderId:       item.LastMessage.SenderId,
				Seq:            item.LastMessage.Seq,
				ConvType:       item.LastMessage.ConvType,
				CType:          int8(item.LastMessage.CType),
				Content:        item.LastMessage.Content,
				SendTime:       item.LastMessage.SendTime,
			}
		}
		if item.PeerUser != nil {
			resItem.PeerUser = &PeerUserRes{
				UserId:   item.PeerUser.UserId,
				UserName: item.PeerUser.UserName,
				NickName: item.PeerUser.NickName,
				Remark:   item.PeerUser.Remark,
				Avatar:   item.PeerUser.Avatar,
			}
		}
		if item.Room != nil {
			resItem.Room = &RoomRes{
				RoomId:      item.Room.RoomId,
				RoomName:    item.Room.RoomName,
				Avatar:      item.Room.Avatar,
				Description: item.Room.Description,
				MemberCount: item.Room.MemberCount,
			}
		}

		res = append(res, resItem)
	}

	return res
}
