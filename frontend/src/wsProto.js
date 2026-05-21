import protobuf from "protobufjs";

const schema = `
syntax = "proto3";
package im.ws;

message WsFrame {
  string op = 1;
  bytes data = 2;
}

message MessageReadAckReq {
  string conversation_id = 1;
  int64 last_read_seq = 2;
}

message MessageReq {
  string client_msg_id = 1;
  string recv_id = 2;
  int32 conv_type = 3;
  int32 c_type = 4;
  string content = 5;
  int64 video_time = 6;
  bool has_video_time = 7;
}

message MessageAck {
  string client_msg_id = 1;
  string message_id = 2;
  string status = 3;
  string extra = 4;
  int64 send_time = 5;
}

message MessageReadAckEvent {
  string user_id = 1;
  string conversation_id = 2;
  int64 last_read_seq = 3;
}

message MessageEvent {
  string message_id = 1;
  string conversation_id = 2;
  string send_id = 3;
  string recv_id = 4;
  int64 seq = 5;
  int32 conv_type = 6;
  int32 c_type = 7;
  string content = 8;
  int64 send_time = 9;
  string sender_username = 10;
}

message WatchVideoControl {
  string room_id = 1;
  string action = 2;
  string video_id = 3;
  string video_url = 4;
  int64 position_ms = 5;
  int64 delta_ms = 6;
  int64 duration_ms = 7;
  double playback_rate = 8;
  int64 client_time_ms = 9;
}

message WatchVideoState {
  string room_id = 1;
  string action = 2;
  string video_id = 3;
  string video_url = 4;
  int64 position_ms = 5;
  int64 delta_ms = 6;
  int64 duration_ms = 7;
  double playback_rate = 8;
  bool is_playing = 9;
  string updated_by = 10;
  int64 updated_at_ms = 11;
  int64 client_time_ms = 12;
}
`;

const root = protobuf.parse(schema).root;
const types = {
  frame: root.lookupType("im.ws.WsFrame"),
  messageReq: root.lookupType("im.ws.MessageReq"),
  messageAck: root.lookupType("im.ws.MessageAck"),
  messageEvent: root.lookupType("im.ws.MessageEvent"),
  readAck: root.lookupType("im.ws.MessageReadAckReq"),
  readAckEvent: root.lookupType("im.ws.MessageReadAckEvent"),
  watchControl: root.lookupType("im.ws.WatchVideoControl"),
  watchState: root.lookupType("im.ws.WatchVideoState"),
};

function toPlain(type, message) {
  return type.toObject(message, {
    longs: Number,
    enums: String,
    bytes: Uint8Array,
    defaults: true,
  });
}

export function encodeFrame(op, typeName, payload) {
  const type = types[typeName];
  const data = type.encode(type.create(payload)).finish();
  return types.frame.encode(types.frame.create({ op, data })).finish();
}

export function decodeFrame(buffer) {
  const frame = types.frame.decode(new Uint8Array(buffer));
  return toPlain(types.frame, frame);
}

export function decodePayload(typeName, data) {
  const type = types[typeName];
  return toPlain(type, type.decode(data));
}
