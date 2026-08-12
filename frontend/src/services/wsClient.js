import { decodeFrame, decodePayload, encodeFrame } from "../wsProto.js";

const payloadTypes = {
  msg: "messageEvent",
  msg_ack: "messageAck",
  msg_read_notify: "readAckEvent",
  room_msg_notice: "roomMessageNotice",
};

const DEVICE_ID_KEY = "im_device_id";

const getDeviceId = () => {
  try {
    const existing = window.localStorage.getItem(DEVICE_ID_KEY);
    if (existing) return existing;
    const generated = globalThis.crypto?.randomUUID?.()
      || `web-${Date.now()}-${Math.random().toString(36).slice(2)}`;
    window.localStorage.setItem(DEVICE_ID_KEY, generated);
    return generated;
  } catch {
    return `web-${Date.now()}-${Math.random().toString(36).slice(2)}`;
  }
};

export function createWsClient(callbacks = {}) {
  let socket = null;
  let authenticated = false;
  let reconnectTimer = 0;
  let reconnectAttempts = 0;
  let manualClose = false;

  const notifyState = (state) => callbacks.onStateChange?.(state);

  const buildUrl = () => {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const params = new URLSearchParams({
      device_id: getDeviceId(),
      platform: "web",
    });
    return `${protocol}//${window.location.host}/api/v1/ws?${params.toString()}`;
  };

  const dispatchApplicationFrame = (frame) => {
    const payloadType = payloadTypes[frame.op];
    if (!payloadType) {
      callbacks.onUnknownFrame?.(frame);
      return;
    }
    const payload = decodePayload(payloadType, frame.data);
    if (frame.op === "msg") callbacks.onMessage?.(payload);
    if (frame.op === "msg_ack") callbacks.onAck?.(payload);
    if (frame.op === "msg_read_notify") callbacks.onReadNotify?.(payload);
    if (frame.op === "room_msg_notice") callbacks.onRoomMessageNotice?.(payload);
  };

  const dispatchFrame = (event) => {
    try {
      const frame = decodeFrame(event.data);
      if (frame.op === "msg_batch") {
        const batch = decodePayload("batch", frame.data);
        batch.frames.forEach(dispatchApplicationFrame);
      } else {
        dispatchApplicationFrame(frame);
      }
    } catch (error) {
      notifyState("error");
      callbacks.onProtocolError?.(error);
    }
  };

  const open = () => {
    if (!authenticated || typeof window === "undefined") return;
    window.clearTimeout(reconnectTimer);
    if (socket) {
      socket.onclose = null;
      socket.close();
    }
    notifyState("connecting");
    socket = new WebSocket(buildUrl());
    socket.binaryType = "arraybuffer";
    socket.onopen = () => {
      const recovered = reconnectAttempts > 0;
      reconnectAttempts = 0;
      notifyState("connected");
      callbacks.onOpen?.({ recovered });
    };
    socket.onmessage = dispatchFrame;
    socket.onerror = (error) => {
      notifyState("error");
      callbacks.onError?.(error);
    };
    socket.onclose = () => {
      if (manualClose || !authenticated) return;
      notifyState("disconnected");
      reconnectAttempts += 1;
      reconnectTimer = window.setTimeout(open, Math.min(reconnectAttempts * 1000, 5000));
    };
  };

  const connect = (nextAuthenticated = true) => {
    authenticated = Boolean(nextAuthenticated);
    manualClose = false;
    open();
  };

  const disconnect = () => {
    manualClose = true;
    authenticated = false;
    window.clearTimeout(reconnectTimer);
    if (socket) {
      socket.onclose = null;
      socket.close();
      socket = null;
    }
    notifyState("disconnected");
  };

  const send = (op, typeName, payload) => {
    if (!socket || socket.readyState !== WebSocket.OPEN) return false;
    socket.send(encodeFrame(op, typeName, payload));
    return true;
  };

  return {
    connect,
    disconnect,
    reconnect: open,
    isConnected: () => socket?.readyState === WebSocket.OPEN,
    sendMessage: (payload) => send("msg", "messageReq", payload),
    sendReadAck: (payload) => send("msg_read_ack", "readAck", payload),
  };
}
