import protobuf from "protobufjs";

const schema = `
syntax = "proto3";
package im.ws;
message WsFrame { string op = 1; bytes data = 2; }
message WsBatch { repeated WsFrame frames = 1; }
message MessageReadAckReq { string conversation_id = 1; int64 last_read_seq = 2; }
message MessageReq {
  string client_msg_id = 1; string recv_id = 2; int32 conv_type = 3; int32 c_type = 4;
  string content = 5; int64 video_time = 6; bool has_video_time = 7;
  string file_id = 10; int32 width = 14; int32 height = 15; int64 duration_ms = 16;
  string sticker_id = 17; string pack_id = 18;
}
message MessageAck { string client_msg_id = 1; string message_id = 2; string status = 3; string extra = 4; int64 send_time = 5; string conversation_id = 6; int64 seq = 7; string attachment_id = 8; }
message RoomMessageNotice { string conversation_id = 1; string message_id = 2; int64 seq = 3; }
message MessageReadAckEvent { string user_id = 1; string conversation_id = 2; int64 last_read_seq = 3; int32 conv_type = 4; string sender_id = 5; string avatar = 6; }
message MessageEvent {
  string message_id = 1; string conversation_id = 2; string send_id = 3; string recv_id = 4;
  int64 seq = 5; int32 conv_type = 6; int32 c_type = 7; string content = 8; int64 send_time = 9;
  string sender_username = 10; string client_msg_id = 11;
  int32 width = 18; int32 height = 19; int64 duration_ms = 20; string sticker_id = 21;
  string pack_id = 22; bool has_video_time = 23; int64 video_time = 24; string attachment_id = 25;
}
`;

const root = protobuf.parse(schema).root;
const types = {
  frame: root.lookupType("im.ws.WsFrame"),
  batch: root.lookupType("im.ws.WsBatch"),
  messageReq: root.lookupType("im.ws.MessageReq"),
  messageAck: root.lookupType("im.ws.MessageAck"),
  roomMessageNotice: root.lookupType("im.ws.RoomMessageNotice"),
  messageEvent: root.lookupType("im.ws.MessageEvent"),
  readAck: root.lookupType("im.ws.MessageReadAckReq"),
  readAckEvent: root.lookupType("im.ws.MessageReadAckEvent"),
};

const toPlain = (type, message) =>
  type.toObject(message, { longs: Number, enums: String, bytes: Uint8Array, defaults: true });

export function encodeFrame(op, typeName, payload) {
  const type = types[typeName];
  const data = type.encode(type.create(payload)).finish();
  return types.frame.encode(types.frame.create({ op, data })).finish();
}

export function decodeFrame(buffer) {
  return toPlain(types.frame, types.frame.decode(new Uint8Array(buffer)));
}

export function decodePayload(typeName, data) {
  const type = types[typeName];
  return toPlain(type, type.decode(data));
}
