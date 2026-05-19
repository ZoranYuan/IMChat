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
		got.GetConvType() != int32(event.ConvType) ||
		got.GetContent() != event.Content {
		t.Fatalf("unexpected protobuf event: messageId=%q conversationId=%q convType=%d content=%q",
			got.GetMessageId(), got.GetConversationId(), got.GetConvType(), got.GetContent())
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

func TestUpsertWatchVideoStateBoundsAndPlayback(t *testing.T) {
	gateway := NewGateway()

	state := gateway.UpsertWatchVideoState(WatchVideoControlReq{
		RoomId:       "room-1",
		Action:       "load",
		VideoId:      "video-1",
		VideoURL:     "https://example.com/video.mp4",
		PositionMs:   1000,
		DurationMs:   10_000,
		PlaybackRate: 1.25,
	}, "u1")
	if state.PositionMs != 0 || state.IsPlaying {
		t.Fatalf("load should reset position and pause playback: %+v", state)
	}
	if state.PlaybackRate != 1.25 {
		t.Fatalf("expected playback rate 1.25, got %v", state.PlaybackRate)
	}

	state = gateway.UpsertWatchVideoState(WatchVideoControlReq{
		RoomId:     "room-1",
		Action:     "forward",
		PositionMs: 9_500,
		DeltaMs:    1_000,
	}, "u2")
	if state.PositionMs != 10_000 {
		t.Fatalf("forward should clamp to duration, got %d", state.PositionMs)
	}

	state = gateway.UpsertWatchVideoState(WatchVideoControlReq{
		RoomId:     "room-1",
		Action:     "backward",
		PositionMs: 200,
		DeltaMs:    500,
	}, "u2")
	if state.PositionMs != 0 {
		t.Fatalf("backward should clamp to zero, got %d", state.PositionMs)
	}

	state = gateway.UpsertWatchVideoState(WatchVideoControlReq{
		RoomId: "room-1",
		Action: "play",
	}, "u3")
	if !state.IsPlaying || state.PlaybackRate != 1.25 {
		t.Fatalf("play should start playback and keep previous playback rate: %+v", state)
	}
}
