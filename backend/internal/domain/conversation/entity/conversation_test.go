package entity

import (
	conversationvo "IM_backend/internal/domain/conversation/value_object"
	"testing"
)

func TestNewConversation(t *testing.T) {
	t.Run("私聊保存双方用户", func(t *testing.T) {
		conversation := NewConversation("c1", "u1", "u2", int(conversationvo.PrivateChat), 0, "")

		if conversation.UserId1 != "u1" || conversation.UserId2 != "u2" || conversation.RoomId != "" {
			t.Fatalf("私聊会话参与者错误: %+v", conversation)
		}
	})

	t.Run("群聊保存目标房间", func(t *testing.T) {
		conversation := NewConversation("r1", "u1", "r1", int(conversationvo.RoomChat), 0, "")

		if conversation.UserId1 != "u1" || conversation.UserId2 != "" || conversation.RoomId != "r1" {
			t.Fatalf("群聊会话参与者错误: %+v", conversation)
		}
	})
}

func TestGetConversationID(t *testing.T) {
	if first := GetConversationID("u2", "u1", int(conversationvo.PrivateChat)); first != "u2_u1" {
		t.Fatalf("私聊会话 ID 错误: %s", first)
	}
	if second := GetConversationID("u1", "u2", int(conversationvo.PrivateChat)); second != "u2_u1" {
		t.Fatalf("私聊会话 ID 应与发送方向无关: %s", second)
	}
	if room := GetConversationID("u1", "r1", int(conversationvo.RoomChat)); room != "r1" {
		t.Fatalf("群聊会话 ID 错误: %s", room)
	}
}
