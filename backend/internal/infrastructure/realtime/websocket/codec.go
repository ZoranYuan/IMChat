package websocket

import (
	"encoding/json"

	"IM_backend/internal/shared/protocol"
	wspb "IM_backend/internal/transport/ws/pb"

	"google.golang.org/protobuf/proto"
)

func EncodePayload(eventType string, payload []byte) ([]byte, error) {
	switch eventType {
	case string(protocol.EventTypeSendMessage):
		var event protocol.MessageEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return proto.Marshal(messageEventToPB(event))
	case string(protocol.EventTypeMsgAck):
		var event protocol.MessageAckEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return proto.Marshal(messageAckToPB(event))
	case string(protocol.EventReadMessageAck), string(protocol.EventReadMessageNotify):
		var event protocol.MessageReadAckEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return proto.Marshal(messageReadAckEventToPB(event))
	case string(protocol.EventRoomMessageNotice):
		var event protocol.MessageNotifyEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		return proto.Marshal(roomMessageNoticeToPB(event))
	default:
		return payload, nil
	}
}

func messageAckToPB(event protocol.MessageAckEvent) *wspb.MessageAck {
	return &wspb.MessageAck{
		ClientMsgId:    event.ClientMsgId,
		MessageId:      event.MessageId,
		Status:         string(event.Status),
		Extra:          event.Extra,
		SendTime:       event.SendTime,
		ConversationId: event.ConversationId,
		Seq:            event.Seq,
		AttachmentId:   event.AttachmentId,
	}
}

func messageReadAckEventToPB(event protocol.MessageReadAckEvent) *wspb.MessageReadAckEvent {
	return &wspb.MessageReadAckEvent{UserId: event.UserId, ConversationId: event.ConversationId, LastReadSeq: event.LastReadSeq, ConvType: int32(event.ConvType), SenderId: event.SenderId, Avatar: event.Avatar}
}

func roomMessageNoticeToPB(event protocol.MessageNotifyEvent) *wspb.RoomMessageNotice {
	return &wspb.RoomMessageNotice{
		ConversationId: event.ConversationId,
		MessageId:      event.MessageId,
		Seq:            event.Seq,
	}
}

func messageEventToPB(event protocol.MessageEvent) *wspb.MessageEvent {
	var videoTime, duration int64
	if event.VideoTime != nil {
		videoTime = *event.VideoTime
	}
	if event.DurationMs != nil {
		duration = *event.DurationMs
	}
	return &wspb.MessageEvent{
		MessageId: event.MessageId, ConversationId: event.ConversationId, SendId: event.SendId,
		SenderUsername: event.SenderUsername, RecvId: event.RecvId, Seq: event.Seq,
		ConvType: int32(event.ConvType), CType: int32(event.CType), Content: event.Content,
		SendTime: event.SendTime, ClientMsgId: event.ClientMsgId,
		Width: int32(event.Width), Height: int32(event.Height),
		DurationMs: duration, StickerId: event.StickerId, PackId: event.PackId,
		HasVideoTime: event.HasVideoTime, VideoTime: videoTime, AttachmentId: event.AttachmentId,
	}
}
