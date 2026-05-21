import { computed, nextTick, onBeforeUnmount, reactive, ref } from "vue";
import {
  createRoom,
  createFriendRequest,
  getDanmaku,
  getFile,
  getFriendRequests,
  getFriends,
  getHistoryMessages,
  getInviteCode,
  getOfflineMessages,
  joinRoom,
  login,
  operateFriendRequest,
  register,
  uploadFile,
} from "../api";
import { decodeFrame, decodePayload, encodeFrame } from "../wsProto";

export function useImClient() {
  const token = ref(localStorage.getItem("im_token") || "");
  const currentUser = reactive(JSON.parse(localStorage.getItem("im_user") || "{}"));
  const authMode = ref("login");
  const authForm = reactive({ phone: "", password: "" });
  const conversations = ref([]);
  const activeConversation = ref(null);
  const messages = ref([]);
  const friends = ref([]);
  const friendRequests = ref([]);
  const friendForm = reactive({ toUserId: "", message: "你好，我想加你为好友" });
  const messageText = ref("");
  const messageList = ref(null);
  const ws = ref(null);
  const wsConnected = ref(false);
  const toast = ref("");

  const roomForm = reactive({ roomName: "一起看房间", inviteCode: "", inviteCodeDisplay: "" });
  const activeRoomId = ref("");
  const fileIdInput = ref("");
  const uploadName = ref("");
  const video = reactive({ fileId: "", url: "", objectKey: "" });
  const videoRef = ref(null);
  const danmakuItems = ref([]);
  const currentVideoTime = ref(0);

  const visibleDanmaku = computed(() => {
    const nowMs = currentVideoTime.value * 1000;
    return danmakuItems.value
      .filter((item) => Math.abs(item.timeMs - nowMs) < 4500)
      .slice(-8)
      .map((item, index) => ({
        ...item,
        top: 12 + (index % 6) * 12,
        duration: 9 + (index % 3),
      }));
  });

  function showToast(message) {
    toast.value = message;
    window.clearTimeout(showToast.timer);
    showToast.timer = window.setTimeout(() => {
      toast.value = "";
    }, 2400);
  }

  async function submitAuth() {
    try {
      const data = authMode.value === "login" ? await login(authForm) : await register(authForm);
      token.value = data.token;
      Object.assign(currentUser, data);
      localStorage.setItem("im_token", data.token);
      localStorage.setItem("im_user", JSON.stringify(data));
      showToast("登录成功");
      await Promise.all([loadOffline(), loadFriends(), loadFriendRequests()]);
      connectWs();
    } catch (err) {
      showToast(err.message);
    }
  }

  async function loadOffline() {
    if (!token.value) return;
    const data = await getOfflineMessages(token.value).catch((err) => {
      showToast(err.message);
      return [];
    });
    conversations.value = (Array.isArray(data) ? data : []).map((item) => ({
      conversationId: item.conversationId,
      unread: item.unread,
      latestMessage: item.latestMessage,
      convType: item.latestMessage?.convType || 2,
    }));
  }

  async function loadFriends() {
    if (!token.value) return;
    const data = await getFriends(token.value).catch((err) => {
      showToast(err.message);
      return [];
    });
    friends.value = Array.isArray(data) ? data : [];
  }

  async function loadFriendRequests() {
    if (!token.value) return;
    const data = await getFriendRequests(token.value).catch((err) => {
      showToast(err.message);
      return [];
    });
    friendRequests.value = Array.isArray(data) ? data : [];
  }

  async function selectConversation(item) {
    activeConversation.value = item;
    if (item.convType === 2) activeRoomId.value = item.conversationId;
    const history = await getHistoryMessages(token.value, item.conversationId).catch((err) => {
      showToast(err.message);
      return { messages: [] };
    });
    messages.value = history.messages || [];
    scrollToBottom();
    if (item.convType === 2) loadDanmaku();
  }

  function openConversation(conversationId, convType, content = "暂无消息") {
    if (!conversationId) return;
    const item = {
      conversationId,
      unread: 0,
      latestMessage: { content, convType },
      convType,
    };
    conversations.value = [item, ...conversations.value.filter((v) => v.conversationId !== item.conversationId)];
    selectConversation(item);
  }

  function openPrivateConversation(friend) {
    const conversationId = friend.friendUserId || friend.toUserId;
    if (!conversationId) return;
    const item = {
      conversationId,
      unread: 0,
      latestMessage: { content: friend.displayName || "好友私聊", convType: 1 },
      convType: 1,
    };
    conversations.value = [item, ...conversations.value.filter((v) => v.conversationId !== item.conversationId)];
    selectConversation(item);
  }

  async function submitFriendRequest() {
    if (!friendForm.toUserId) {
      showToast("请输入用户 ID");
      return;
    }
    try {
      await createFriendRequest(token.value, friendForm);
      friendForm.toUserId = "";
      showToast("好友申请已发送");
    } catch (err) {
      showToast(err.message);
    }
  }

  async function handleFriendRequest(requestId, action) {
    try {
      await operateFriendRequest(token.value, requestId, action);
      showToast(action === 1 ? "已同意好友申请" : "已拒绝好友申请");
      await Promise.all([loadFriends(), loadFriendRequests()]);
    } catch (err) {
      showToast(err.message);
    }
  }

  function connectWs() {
    if (!token.value) {
      showToast("请先登录");
      return;
    }
    if (ws.value) ws.value.close();
    const protocol = window.location.protocol === "https:" ? "wss" : "ws";
    const url = `${protocol}://${window.location.host}/api/v1/ws?token=${encodeURIComponent(token.value)}`;
    ws.value = new WebSocket(url);
    ws.value.binaryType = "arraybuffer";
    ws.value.onopen = () => {
      wsConnected.value = true;
      showToast("实时通道已连接");
    };
    ws.value.onclose = () => {
      wsConnected.value = false;
    };
    ws.value.onerror = () => showToast("WebSocket 连接异常");
    ws.value.onmessage = handleWsMessage;
  }

  function logout() {
    if (ws.value) {
      ws.value.close();
      ws.value = null;
    }
    token.value = "";
    wsConnected.value = false;
    conversations.value = [];
    friends.value = [];
    friendRequests.value = [];
    activeConversation.value = null;
    messages.value = [];
    activeRoomId.value = "";
    localStorage.removeItem("im_token");
    localStorage.removeItem("im_user");
    for (const key of Object.keys(currentUser)) {
      delete currentUser[key];
    }
  }

  function handleWsMessage(event) {
    try {
      const frame = decodeFrame(event.data);
      if (frame.op === "msg") {
        const msg = decodePayload("messageEvent", frame.data);
        if (activeConversation.value?.conversationId === msg.conversationId) {
          messages.value.push({
            messageId: msg.messageId,
            conversationId: msg.conversationId,
            senderId: msg.sendId,
            content: msg.content,
            seq: msg.seq,
            convType: msg.convType,
            cType: msg.cType,
            sendTime: msg.sendTime || Date.now(),
          });
          scrollToBottom();
        }
      }
      if (frame.op === "msg_ack") {
        const ack = decodePayload("messageAck", frame.data);
        if (ack.status === "failed") showToast(ack.extra || "消息发送失败");
      }
      if (frame.op === "watch_video_sync") {
        applyWatchState(decodePayload("watchState", frame.data));
      }
    } catch (err) {
      showToast(`消息解析失败：${err.message}`);
    }
  }

  function sendFrame(op, typeName, payload) {
    if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
      showToast("WebSocket 未连接");
      return false;
    }
    ws.value.send(encodeFrame(op, typeName, payload));
    return true;
  }

  function sendMessage() {
    const content = messageText.value.trim();
    if (!content || !activeConversation.value) return;
    const clientMsgId = crypto.randomUUID();
    const videoTime = videoRef.value ? Math.floor(videoRef.value.currentTime * 1000) : 0;
    const ok = sendFrame("msg", "messageReq", {
      clientMsgId,
      recvId: activeConversation.value.conversationId,
      convType: activeConversation.value.convType || 2,
      cType: 1,
      content,
      videoTime,
      hasVideoTime: Boolean(activeRoomId.value && video.url),
    });
    if (!ok) return;
    messages.value.push({
      clientMsgId,
      senderId: currentUser.userId,
      content,
      sendTime: Date.now(),
      convType: activeConversation.value.convType,
    });
    messageText.value = "";
    scrollToBottom();
  }

  async function handleCreateRoom() {
    try {
      const data = await createRoom(token.value, roomForm);
      activeRoomId.value = data.roomId;
      openConversation(data.roomId, 2, "一起看房间");
      roomForm.inviteCodeDisplay = data.inviteCode || "";
      showToast("房间已创建");
    } catch (err) {
      showToast(err.message);
    }
  }

  async function handleJoinRoom() {
    try {
      const data = await joinRoom(token.value, roomForm.inviteCode);
      activeRoomId.value = data.room?.roomId || data.roomId || "";
      if (activeRoomId.value) {
        openConversation(activeRoomId.value, 2, "一起看房间");
      }
      showToast("已加入房间");
    } catch (err) {
      showToast(err.message);
    }
  }

  async function handleInvite() {
    try {
      const data = await getInviteCode(token.value, activeRoomId.value);
      roomForm.inviteCodeDisplay = typeof data === "string" ? data : data.inviteCode;
    } catch (err) {
      showToast(err.message);
    }
  }

  async function handleUpload(event) {
    const file = event.target.files?.[0];
    if (!file) return;
    uploadName.value = file.name;
    try {
      const data = await uploadFile(token.value, file);
      Object.assign(video, { fileId: data.fileId, url: data.url, objectKey: data.objectKey });
      fileIdInput.value = data.fileId;
      showToast("视频上传完成");
    } catch (err) {
      showToast(err.message);
    }
  }

  async function loadFile() {
    try {
      const data = await getFile(token.value, fileIdInput.value);
      Object.assign(video, { fileId: data.fileId, url: data.url, objectKey: data.objectKey });
      showToast("视频已加载");
    } catch (err) {
      showToast(err.message);
    }
  }

  function loadVideoToRoom() {
    sendWatchControl("load", { positionMs: 0 });
  }

  function sendWatchControl(action, patch = {}) {
    if (!activeRoomId.value) {
      showToast("请先进入房间");
      return;
    }
    const current = videoRef.value ? Math.floor(videoRef.value.currentTime * 1000) : 0;
    sendFrame("watch_video_control", "watchControl", {
      roomId: activeRoomId.value,
      action,
      videoId: video.fileId || activeRoomId.value,
      videoUrl: video.url,
      positionMs: current,
      durationMs: videoRef.value ? Math.floor((videoRef.value.duration || 0) * 1000) : 0,
      playbackRate: videoRef.value?.playbackRate || 1,
      clientTimeMs: Date.now(),
      ...patch,
    });
  }

  function seekBy(deltaMs) {
    if (videoRef.value) videoRef.value.currentTime = Math.max(0, videoRef.value.currentTime + deltaMs / 1000);
    sendWatchControl(deltaMs > 0 ? "forward" : "backward", { deltaMs: Math.abs(deltaMs) });
  }

  function applyWatchState(state) {
    if (state.roomId && state.roomId !== activeRoomId.value) return;
    if (state.videoUrl) video.url = state.videoUrl;
    nextTick(() => {
      if (!videoRef.value) return;
      const target = (state.positionMs || 0) / 1000;
      if (Math.abs(videoRef.value.currentTime - target) > 1.2) videoRef.value.currentTime = target;
      videoRef.value.playbackRate = state.playbackRate || 1;
      if (state.isPlaying) videoRef.value.play().catch(() => {});
      else videoRef.value.pause();
    });
  }

  async function loadDanmaku() {
    if (!activeRoomId.value || !token.value) return;
    const data = await getDanmaku(token.value, activeRoomId.value).catch(() => ({ items: [] }));
    danmakuItems.value = data.items || [];
  }

  function onVideoTimeUpdate() {
    currentVideoTime.value = videoRef.value?.currentTime || 0;
  }

  function scrollToBottom() {
    nextTick(() => {
      if (messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight;
    });
  }

  function formatTime(ts) {
    if (!ts) return "刚刚";
    return new Date(ts).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  }

  onBeforeUnmount(() => {
    if (ws.value) ws.value.close();
  });

  return {
    token,
    currentUser,
    authMode,
    authForm,
    conversations,
    activeConversation,
    messages,
    friends,
    friendRequests,
    friendForm,
    messageText,
    messageList,
    wsConnected,
    toast,
    roomForm,
    activeRoomId,
    fileIdInput,
    uploadName,
    video,
    videoRef,
    visibleDanmaku,
    submitAuth,
    loadOffline,
    loadFriends,
    loadFriendRequests,
    selectConversation,
    openPrivateConversation,
    submitFriendRequest,
    handleFriendRequest,
    connectWs,
    logout,
    sendMessage,
    handleCreateRoom,
    handleJoinRoom,
    handleInvite,
    handleUpload,
    loadFile,
    loadVideoToRoom,
    sendWatchControl,
    seekBy,
    onVideoTimeUpdate,
    formatTime,
  };
}
