package websocket

import (
	"encoding/json"

	"IM_backend/internal/shared/protocol"
	wspb "IM_backend/internal/transport/ws/pb"

	"google.golang.org/protobuf/proto"
)

// EncodePayload 按事件类型将后端 JSON 事件编码为对应的 WebSocket protobuf payload。
// 未配置 protobuf 映射的事件保持原始 payload，保证非消息事件可以继续透传。
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

// messageAckToPB 将内部 ACK 事件编码为前端使用的 protobuf 消息。
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

// messageReadAckEventToPB 将内部已读事件编码为前端使用的 protobuf 消息。
func messageReadAckEventToPB(event protocol.MessageReadAckEvent) *wspb.MessageReadAckEvent {
	return &wspb.MessageReadAckEvent{UserId: event.UserId, ConversationId: event.ConversationId, LastReadSeq: event.LastReadSeq, ConvType: int32(event.ConvType), SenderId: event.SenderId, Avatar: event.Avatar}
}

// roomMessageNoticeToPB 将大群轻量通知编码为 protobuf。
// 通知只携带会话和序号，客户端随后通过同步接口拉取消息详情。
func roomMessageNoticeToPB(event protocol.MessageNotifyEvent) *wspb.RoomMessageNotice {
	return &wspb.RoomMessageNotice{
		ConversationId: event.ConversationId,
		MessageId:      event.MessageId,
		Seq:            event.Seq,
	}
}

// messageEventToPB 将内部消息事件映射为只包含消息本体的 WebSocket 推送对象。
// Kafka 投递所需的路由字段仍保留在内部事件中，但不会进入前端协议。
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
