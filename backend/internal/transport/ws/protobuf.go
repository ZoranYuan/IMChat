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
	var durationMs *int64
	if v := pb.GetDurationMs(); v > 0 {
		durationMs = &v
	}

	return MessageReq{
		ClientMsgId: pb.GetClientMsgId(),
		RecvId:      pb.GetRecvId(),
		ConvType:    int(pb.GetConvType()),
		CType:       int(pb.GetCType()),
		Content:     pb.GetContent(),
		MediaURL:    pb.GetMediaUrl(),
		ThumbURL:    pb.GetThumbUrl(),
		FileId:      pb.GetFileId(),
		ThumbFileId: pb.GetThumbFileId(),
		FileName:    pb.GetFileName(),
		FileSize:    pb.GetFileSize(),
		Width:       int(pb.GetWidth()),
		Height:      int(pb.GetHeight()),
		DurationMs:  durationMs,
		StickerId:   pb.GetStickerId(),
		PackId:      pb.GetPackId(),
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
		ConvType:       int32(event.ConvType),
		SenderId:       event.SenderId,
		Avatar:         event.Avatar,
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
		MediaUrl:       event.MediaURL,
		ThumbUrl:       event.ThumbURL,
		FileId:         event.FileId,
		ThumbFileId:    event.ThumbFileId,
		FileName:       event.FileName,
		FileSize:       event.FileSize,
		Width:          int32(event.Width),
		Height:         int32(event.Height),
		DurationMs: func() int64 {
			if event.DurationMs != nil {
				return *event.DurationMs
			}
			return 0
		}(),
		StickerId:    event.StickerId,
		PackId:       event.PackId,
		HasVideoTime: event.HasVideoTime,
		VideoTime:    videoTime,
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
