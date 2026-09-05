package ws

import (
	realtimews "IM_backend/internal/infrastructure/realtime/websocket"
	"IM_backend/internal/shared/protocol"
	wspb "IM_backend/internal/transport/ws/pb"
)

// encodeWebSocketPayload 将业务事件交给统一的 WebSocket protobuf 编码器处理。
func encodeWebSocketPayload(op string, payload []byte) ([]byte, error) {
	return realtimews.EncodePayload(op, payload)
}

// messageReqFromPB 将 WebSocket 层的 protobuf 请求转换为业务层发送请求。
// 请求中的接收方、会话类型和文件标识等控制字段只用于服务端处理，不属于推送消息本体。
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
		FileId:      pb.GetFileId(),
		Width:       int(pb.GetWidth()),
		Height:      int(pb.GetHeight()),
		DurationMs:  durationMs,
		StickerId:   pb.GetStickerId(),
		PackId:      pb.GetPackId(),
		VideoTime:   videoTime,
	}
}

// messageAckToPB 将业务层生成的消息 ACK 转换为 WebSocket protobuf 响应。
func messageAckToPB(event protocol.MessageAckEvent) *wspb.MessageAck {
	return &wspb.MessageAck{
		ClientMsgId:  event.ClientMsgId,
		MessageId:    event.MessageId,
		Status:       string(event.Status),
		Extra:        event.Extra,
		SendTime:     event.SendTime,
		AttachmentId: event.AttachmentId,
	}
}

// messageReadAckEventToPB 将已读回执事件转换为 WebSocket protobuf 响应。
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

// messageEventToPB 将内部消息事件转换为只包含消息本体字段的 WebSocket 推送对象。
// 内部事件中的路由、用户展示和媒体子表字段不会通过 MessageEvent 下发给前端。
func messageEventToPB(event protocol.MessageEvent) *wspb.MessageEvent {
	return &wspb.MessageEvent{
		MessageId:      event.MessageId,
		ConversationId: event.ConversationId,
		SenderId:       event.SenderId,
		Seq:            event.Seq,
		CType:          int32(event.CType),
		Content:        event.Content,
		SendTime:       event.SendTime,
		ClientMsgId:    event.ClientMsgId,
		VideoId:        event.VideoId,
		VideoTime:      event.VideoTime,
		Status:         int32(event.Status),
		AttachmentId:   event.AttachmentId,
	}
}
