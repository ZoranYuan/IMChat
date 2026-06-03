package ws

import (
	"IM_backend/internal/shared/protocol"
	wspb "IM_backend/internal/transport/ws/pb"
)

func watchVideoControlFromPB(pb *wspb.WatchVideoControl) WatchVideoControlReq {
	return WatchVideoControlReq{
		RoomId:       pb.GetRoomId(),
		Action:       pb.GetAction(),
		VideoId:      pb.GetVideoId(),
		VideoURL:     pb.GetVideoUrl(),
		PositionMs:   pb.GetPositionMs(),
		DeltaMs:      pb.GetDeltaMs(),
		DurationMs:   pb.GetDurationMs(),
		PlaybackRate: pb.GetPlaybackRate(),
		ClientTimeMs: pb.GetClientTimeMs(),
	}
}

func messageReqFromPB(pb *wspb.MessageReq) MessageReq {
	var videoTime *int64
	if pb.GetHasVideoTime() {
		v := pb.GetVideoTime()
		videoTime = &v
	}

	return MessageReq{
		ClientMsgId: pb.GetClientMsgId(),
		RecvId:      pb.GetRecvId(),
		ConvType:    int(pb.GetConvType()),
		CType:       int(pb.GetCType()),
		Content:     pb.GetContent(),
		VideoTime:   videoTime,
	}
}

func messageAckToPB(event protocol.MessageAckEvent) *wspb.MessageAck {
	return &wspb.MessageAck{
		ClientMsgId: event.ClientMsgId,
		MessageId:   event.MessageId,
		Status:      string(event.Status),
		Extra:       event.Extra,
		SendTime:    event.SendTime,
	}
}

func messageReadAckEventToPB(event protocol.MessageReadAckEvent) *wspb.MessageReadAckEvent {
	return &wspb.MessageReadAckEvent{
		UserId:         event.UserId,
		ConversationId: event.ConversationId,
		LastReadSeq:    event.LastReadSeq,
	}
}

func messageEventToPB(event protocol.MessageEvent) *wspb.MessageEvent {
	var videoTime int64
	if event.VideoTime != nil {
		videoTime = *event.VideoTime
	}
	return &wspb.MessageEvent{
		MessageId:      event.MessageId,
		ConversationId: event.ConversationId,
		SendId:         event.SendId,
		SenderUsername: event.SenderUsername,
		RecvId:         event.RecvId,
		Seq:            event.Seq,
		ConvType:       int32(event.ConvType),
		CType:          int32(event.CType),
		Content:        event.Content,
		SendTime:       event.SendTime,
		ClientMsgId:    event.ClientMsgId,
		HasVideoTime:   event.HasVideoTime,
		VideoTime:      videoTime,
	}
}

func watchVideoStateToPB(state WatchVideoState) *wspb.WatchVideoState {
	return &wspb.WatchVideoState{
		RoomId:       state.RoomId,
		Action:       state.Action,
		VideoId:      state.VideoId,
		VideoUrl:     state.VideoURL,
		PositionMs:   state.PositionMs,
		DeltaMs:      state.DeltaMs,
		DurationMs:   state.DurationMs,
		PlaybackRate: state.PlaybackRate,
		IsPlaying:    state.IsPlaying,
		UpdatedBy:    state.UpdatedBy,
		UpdatedAtMs:  state.UpdatedAtMs,
		ClientTimeMs: state.ClientTimeMs,
	}
}
