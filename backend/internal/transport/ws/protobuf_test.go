package ws

import (
	"IM_backend/internal/shared/protocol"
	wspb "IM_backend/internal/transport/ws/pb"
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestEncodeWebSocketPayloadMessageEvent(t *testing.T) {
	event := protocol.MessageEvent{
		MessageId:      "msg-1",
		ConversationId: "conv-1",
		SendId:         "u1",
		SenderUsername: "alice",
		RecvId:         "u2",
		Seq:            12,
		ConvType:       protocol.RoomChat,
		CType:          1,
		Content:        "hello",
		SendTime:       1710000000000,
	}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	encoded, err := encodeWebSocketPayload(string(protocol.EventTypeMessage), raw)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	var got wspb.MessageEvent
	if err := proto.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("unmarshal protobuf payload: %v", err)
	}
	if got.GetMessageId() != event.MessageId ||
		got.GetConversationId() != event.ConversationId ||
		got.GetSenderUsername() != event.SenderUsername ||
		got.GetConvType() != int32(event.ConvType) ||
		got.GetContent() != event.Content {
		t.Fatalf("unexpected protobuf event: messageId=%q conversationId=%q senderUsername=%q convType=%d content=%q",
			got.GetMessageId(), got.GetConversationId(), got.GetSenderUsername(), got.GetConvType(), got.GetContent())
	}
}

func TestEncodeWebSocketPayloadMessageReadAckEvent(t *testing.T) {
	event := protocol.MessageReadAckEvent{
		UserId:         "u2",
		ConversationId: "conv-1",
		LastReadSeq:    18,
		ConvType:       protocol.RoomChat,
		SenderId:       "u1",
		Avatar:         "https://example.com/avatar.png",
	}
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	encoded, err := encodeWebSocketPayload(string(protocol.EventMessageReadAck), raw)
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	var got wspb.MessageReadAckEvent
	if err := proto.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("unmarshal protobuf payload: %v", err)
	}
	if got.GetUserId() != event.UserId ||
		got.GetConversationId() != event.ConversationId ||
		got.GetLastReadSeq() != event.LastReadSeq ||
		got.GetConvType() != int32(event.ConvType) ||
		got.GetSenderId() != event.SenderId ||
		got.GetAvatar() != event.Avatar {
		t.Fatalf("unexpected protobuf event: userId=%q conversationId=%q lastReadSeq=%d convType=%d senderId=%q avatar=%q",
			got.GetUserId(), got.GetConversationId(), got.GetLastReadSeq(), got.GetConvType(), got.GetSenderId(), got.GetAvatar())
	}
}

func TestMessageReqFromPBVideoTime(t *testing.T) {
	req := messageReqFromPB(&wspb.MessageReq{
		ClientMsgId:  "client-1",
		RecvId:       "room-1",
		ConvType:     2,
		CType:        1,
		Content:      "danmaku",
		VideoTime:    4500,
		HasVideoTime: true,
	})

	if req.VideoTime == nil || *req.VideoTime != 4500 {
		t.Fatalf("expected video time 4500, got %+v", req.VideoTime)
	}

	req = messageReqFromPB(&wspb.MessageReq{VideoTime: 4500})
	if req.VideoTime != nil {
		t.Fatalf("expected nil video time when has_video_time is false")
	}
}
