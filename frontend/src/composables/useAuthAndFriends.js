import { computed, onBeforeUnmount, reactive, ref } from "vue";
import {
  createFriendRequest,
  getFriendRequests,
  getFriends,
  login,
  operateFriendRequest,
  register,
  resolveUser,
} from "../api";
import { decodeFrame, decodePayload, encodeFrame } from "../wsProto";

export function useAuthAndFriends({ showMessage, onWsFrame, onWsReconnect }) {
  const token = ref(localStorage.getItem("im_token") || "");
  const currentUser = reactive(JSON.parse(localStorage.getItem("im_user") || "{}"));
  const authMode = ref("login");
  const authForm = reactive({ account: "", phone: "", password: "" });
  const friends = ref([]);
  const friendRequests = ref([]);
  const friendForm = reactive({ keyword: "", toUserId: "", message: "你好，我想加你为好友" });
  const ws = ref(null);
  const wsConnected = ref(false);
  const wsReconnecting = ref(false);
  const wsReconnectFailed = ref(false);
  const message = ref("");
  const messageType = ref("info");
  const maxWsReconnectAttempts = 5;
  const wsReconnectAttempts = ref(0);
  let wsReconnectTimer = 0;
  let wsManualClose = false;
  let messageTimer = 0;
  let frameHandler = onWsFrame || (() => {});

  const wsStatusText = computed(() => {
    if (wsConnected.value) return "在线";
    if (wsReconnecting.value) return "重连中";
    if (wsReconnectFailed.value) return "连接失败";
    return "未连接到服务器";
  });

  function showLocalMessage(nextMessage, type = "danger") {
    message.value = nextMessage;
    messageType.value = type;
    window.clearTimeout(messageTimer);
    messageTimer = window.setTimeout(() => {
      message.value = "";
      messageType.value = "info";
    }, 2400);
  }

  function setWsFrameHandler(handler) {
    frameHandler = handler || (() => {});
  }

  async function submitAuth() {
    const data = authMode.value === "login" ? await login(authForm) : await register(authForm);
    token.value = data.token;
    Object.assign(currentUser, data);
    localStorage.setItem("im_token", data.token);
    localStorage.setItem("im_user", JSON.stringify(data));
    showLocalMessage("登录成功", "success");
    return data;
  }

  async function loadFriends() {
    if (!token.value) return false;
    const data = await getFriends(token.value).catch((err) => {
      showLocalMessage(err.message);
      return null;
    });
    if (!data) return false;
    friends.value = Array.isArray(data) ? data : [];
    return true;
  }

  async function loadFriendRequests() {
    if (!token.value) return false;
    const data = await getFriendRequests(token.value).catch((err) => {
      showLocalMessage(err.message);
      return null;
    });
    if (!data) return false;
    friendRequests.value = Array.isArray(data) ? data : [];
    return true;
  }

  async function submitFriendRequest() {
    if (!friendForm.keyword) {
      showLocalMessage("请输入用户名或手机号");
      return;
    }
    try {
      const user = await resolveUser(token.value, friendForm.keyword);
      friendForm.toUserId = user.userId;
      await createFriendRequest(token.value, friendForm);
      friendForm.keyword = "";
      friendForm.toUserId = "";
      showLocalMessage("好友申请已发送", "success");
    } catch (err) {
      showLocalMessage(err.message);
    }
  }

  async function handleFriendRequest(requestId, action) {
    try {
      await operateFriendRequest(token.value, requestId, action);
      showLocalMessage(action === 1 ? "已同意好友申请" : "已拒绝好友申请", action === 1 ? "success" : "warning");
      await Promise.all([loadFriends(), loadFriendRequests()]);
    } catch (err) {
      showLocalMessage(err.message);
    }
  }

  function clearWsReconnectTimer() {
    window.clearTimeout(wsReconnectTimer);
    wsReconnectTimer = 0;
  }

  function handleWsMessage(event) {
    try {
      const frame = decodeFrame(event.data);
      if (frame.op === "msg") {
        frameHandler({ op: "msg", payload: decodePayload("messageEvent", frame.data) });
      } else if (frame.op === "msg_ack") {
        frameHandler({ op: "msg_ack", payload: decodePayload("messageAck", frame.data) });
      } else if (frame.op === "watch_video_sync") {
        frameHandler({ op: "watch_video_sync", payload: decodePayload("watchState", frame.data) });
      } else if (frame.op === "msg_read_notify") {
        const raw = new TextDecoder().decode(frame.data);
        frameHandler({ op: "msg_read_notify", payload: JSON.parse(raw) });
      }
    } catch (err) {
      showLocalMessage(`消息解析失败：${err.message}`);
    }
  }

  function connectWs(resetReconnect = false) {
    if (!token.value) {
      showLocalMessage("请先登录");
      return;
    }
    if (resetReconnect) {
      wsReconnectAttempts.value = 0;
      wsReconnectFailed.value = false;
    }
    clearWsReconnectTimer();
    wsManualClose = true;
    if (ws.value) {
      ws.value.onclose = null;
      ws.value.close();
    }
    wsManualClose = false;

    const protocol = window.location.protocol === "https:" ? "wss" : "ws";
    const url = `${protocol}://${window.location.host}/api/v1/ws?token=${encodeURIComponent(token.value)}`;
    ws.value = new WebSocket(url);
    ws.value.binaryType = "arraybuffer";
    ws.value.onopen = () => {
      const wasReconnecting = wsReconnecting.value;
      wsConnected.value = true;
      wsReconnecting.value = false;
      wsReconnectFailed.value = false;
      wsReconnectAttempts.value = 0;
      if (wasReconnecting && typeof onWsReconnect === "function") {
        onWsReconnect();
      }
    };
    ws.value.onclose = () => {
      wsConnected.value = false;
      if (!wsManualClose && token.value) scheduleWsReconnect();
    };
    ws.value.onerror = () => {
      wsConnected.value = false;
    };
    ws.value.onmessage = handleWsMessage;
  }

  function scheduleWsReconnect() {
    if (wsReconnectAttempts.value >= maxWsReconnectAttempts) {
      wsReconnecting.value = false;
      wsReconnectFailed.value = true;
      showLocalMessage("实时通道连接失败，请点击状态点重试");
      return;
    }

    wsReconnectAttempts.value += 1;
    wsReconnecting.value = true;
    wsReconnectFailed.value = false;
    const delay = Math.min(1000 * wsReconnectAttempts.value, 5000);
    clearWsReconnectTimer();
    wsReconnectTimer = window.setTimeout(() => connectWs(false), delay);
  }

  function retryWsConnection() {
    if (wsConnected.value || wsReconnecting.value) return;
    connectWs(true);
  }

  function sendFrame(op, typeName, payload) {
    if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
      showLocalMessage("WebSocket 未连接");
      return false;
    }
    ws.value.send(encodeFrame(op, typeName, payload));
    return true;
  }

  function logout() {
    clearWsReconnectTimer();
    wsManualClose = true;
    if (ws.value) {
      ws.value.onclose = null;
      ws.value.close();
      ws.value = null;
    }
    token.value = "";
    wsConnected.value = false;
    wsReconnecting.value = false;
    wsReconnectFailed.value = false;
    wsReconnectAttempts.value = 0;
    friends.value = [];
    friendRequests.value = [];
    localStorage.removeItem("im_token");
    localStorage.removeItem("im_user");
    for (const key of Object.keys(currentUser)) {
      delete currentUser[key];
    }
  }

  function cleanup() {
    clearWsReconnectTimer();
    window.clearTimeout(messageTimer);
    wsManualClose = true;
    if (ws.value) {
      ws.value.onclose = null;
      ws.value.close();
    }
  }

  onBeforeUnmount(cleanup);

  return {
    token,
    currentUser,
    authMode,
    authForm,
    friends,
    friendRequests,
    friendForm,
    wsConnected,
    wsReconnecting,
    wsReconnectFailed,
    wsStatusText,
    message,
    messageType,
    submitAuth,
    loadFriends,
    loadFriendRequests,
    submitFriendRequest,
    handleFriendRequest,
    connectWs,
    retryWsConnection,
    logout,
    sendFrame,
    setWsFrameHandler,
    showMessage: showLocalMessage,
    cleanup,
  };
}
