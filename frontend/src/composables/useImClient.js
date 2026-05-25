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
  getRoomVideoHistory,
  joinRoom,
  login,
  operateFriendRequest,
  register,
  resolveUser,
} from "../api";
import { decodeFrame, decodePayload, encodeFrame } from "../wsProto";
import { useChunkUpload } from "./useChunkUpload";

export function useImClient() {
  const token = ref(localStorage.getItem("im_token") || "");
  const currentUser = reactive(JSON.parse(localStorage.getItem("im_user") || "{}"));
  const authMode = ref("login");
  const authForm = reactive({ account: "", phone: "", password: "" });
  const conversations = ref([]);
  const activeConversation = ref(null);
  const messages = ref([]);
  const friends = ref([]);
  const friendRequests = ref([]);
  const friendForm = reactive({ keyword: "", toUserId: "", message: "你好，我想加你为好友" });
  const messageText = ref("");
  const messageList = ref(null);
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

  const roomForm = reactive({ roomName: "一起看房间", inviteCode: "", inviteCodeDisplay: "" });
  const activeRoomId = ref("");
  const fileIdInput = ref("");
  const uploadName = ref("");
  const video = reactive({ fileId: "", url: "", objectKey: "" });
  const videoRef = ref(null);
  const danmakuItems = ref([]);
  const roomVideoHistory = ref([]);
  const currentVideoTime = ref(0);
  const chunkUpload = useChunkUpload();
  const applyingWatchState = ref(false);

  function getConversationIndex(conversationId) {
    return conversations.value.findIndex((item) => item.conversationId === conversationId);
  }

  function upsertConversationPreview(conversationId, patch = {}, moveToTop = false) {
    if (!conversationId) return null;

    const index = getConversationIndex(conversationId);
    const existing = index >= 0 ? conversations.value[index] : null;
    const nextUnread = (() => {
      if (typeof patch.unread === "number") return Math.max(0, patch.unread);
      if (typeof patch.unreadDelta === "number") return Math.max(0, (existing?.unread || 0) + patch.unreadDelta);
      if (patch.resetUnread) return 0;
      return existing?.unread || 0;
    })();

    const nextItem = {
      conversationId,
      displayName:
        patch.displayName ??
        existing?.displayName ??
        patch.latestMessage?.displayName ??
        existing?.latestMessage?.displayName ??
        "会话",
      unread: nextUnread,
      latestMessage: patch.latestMessage ?? existing?.latestMessage ?? null,
      convType: patch.convType ?? existing?.convType ?? patch.latestMessage?.convType ?? 2,
    };

    if (existing) {
      Object.assign(existing, nextItem);
      conversations.value = moveToTop
        ? [existing, ...conversations.value.filter((item) => item.conversationId !== conversationId)]
        : conversations.value.map((item) => (item.conversationId === conversationId ? existing : item));
      if (activeConversation.value?.conversationId === conversationId) {
        activeConversation.value = existing;
      }
      return existing;
    }

    conversations.value = moveToTop ? [nextItem, ...conversations.value] : [...conversations.value, nextItem];
    if (activeConversation.value?.conversationId === conversationId) {
      activeConversation.value = nextItem;
    }
    return nextItem;
  }

  function sendReadAck(conversationId, lastReadSeq) {
    if (!conversationId || !lastReadSeq || !ws.value || ws.value.readyState !== WebSocket.OPEN) return;
    sendFrame("msg_read_ack", "readAck", {
      conversationId,
      lastReadSeq,
    });
  }

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

  function showMessage(nextMessage, type = "danger") {
    message.value = nextMessage;
    messageType.value = type;
    window.clearTimeout(showMessage.timer);
    showMessage.timer = window.setTimeout(() => {
      message.value = "";
      messageType.value = "info";
    }, 2400);
  }

  async function submitAuth() {
    try {
      const data = authMode.value === "login" ? await login(authForm) : await register(authForm);
      token.value = data.token;
      Object.assign(currentUser, data);
      localStorage.setItem("im_token", data.token);
      localStorage.setItem("im_user", JSON.stringify(data));
      showMessage("登录成功", "success");
      await Promise.all([loadOffline(), loadFriends(), loadFriendRequests()]);
      connectWs();
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function loadOffline() {
    if (!token.value) return;
    const data = await getOfflineMessages(token.value).catch((err) => {
      showMessage(err.message);
      return [];
    });
    conversations.value = (Array.isArray(data) ? data : []).map((item) => ({
      conversationId: item.conversationId,
      displayName: item.displayName || item.latestMessage?.displayName || "会话",
      unread: Number(item.unread) || 0,
      latestMessage: item.latestMessage,
      convType: item.latestMessage?.convType || 2,
    }));
    if (activeConversation.value?.conversationId) {
      const current = conversations.value.find((item) => item.conversationId === activeConversation.value.conversationId);
      if (current) activeConversation.value = current;
    }
  }

  async function loadFriends() {
    if (!token.value) return;
    const data = await getFriends(token.value).catch((err) => {
      showMessage(err.message);
      return [];
    });
    friends.value = Array.isArray(data) ? data : [];
  }

  async function loadFriendRequests() {
    if (!token.value) return;
    const data = await getFriendRequests(token.value).catch((err) => {
      showMessage(err.message);
      return [];
    });
    friendRequests.value = Array.isArray(data) ? data : [];
  }

  async function selectConversation(item) {
    const current = upsertConversationPreview(item.conversationId, {
      displayName: item.displayName,
      convType: item.convType,
      unread: 0,
      resetUnread: true,
      latestMessage: item.latestMessage || null,
    });
    activeConversation.value = current || item;
    if (activeConversation.value.convType === 2) activeRoomId.value = activeConversation.value.conversationId;
    const history = await getHistoryMessages(token.value, item.conversationId).catch((err) => {
      showMessage(err.message);
      return { messages: [] };
    });
    messages.value = history.messages || [];
    if (item.convType === 2) {
      const lastSeq = history.messages?.at(-1)?.seq || 0;
      sendReadAck(item.conversationId, lastSeq);
    }
    scrollToBottom();
    if (activeConversation.value.convType === 2) {
      loadDanmaku();
      loadRoomVideoHistory();
    } else {
      roomVideoHistory.value = [];
    }
  }

  function openConversation(conversationId, convType, content = "暂无消息") {
    if (!conversationId) return;
    const item = upsertConversationPreview(
      conversationId,
      {
        displayName: content,
        unread: 0,
        resetUnread: true,
        latestMessage: { content, convType },
        convType,
      },
      true,
    );
    selectConversation(item);
  }

  function openPrivateConversation(friend) {
    const conversationId = friend.friendUserId || friend.toUserId;
    if (!conversationId) return;
    const item = upsertConversationPreview(
      conversationId,
      {
        displayName: friend.displayName || friend.friendUsername || friend.username || "好友",
        unread: 0,
        resetUnread: true,
        latestMessage: { content: friend.displayName || "好友私聊", convType: 1 },
        convType: 1,
      },
      true,
    );
    selectConversation(item);
  }

  async function submitFriendRequest() {
    if (!friendForm.keyword) {
      showMessage("请输入用户名或手机号");
      return;
    }
    try {
      const user = await resolveUser(token.value, friendForm.keyword);
      friendForm.toUserId = user.userId;
      await createFriendRequest(token.value, friendForm);
      friendForm.keyword = "";
      friendForm.toUserId = "";
      showMessage("好友申请已发送", "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function handleFriendRequest(requestId, action) {
    try {
      await operateFriendRequest(token.value, requestId, action);
      showMessage(action === 1 ? "已同意好友申请" : "已拒绝好友申请", action === 1 ? "success" : "warning");
      await Promise.all([loadFriends(), loadFriendRequests()]);
    } catch (err) {
      showMessage(err.message);
    }
  }

  const wsStatusText = computed(() => {
    if (wsConnected.value) return "在线";
    if (wsReconnecting.value) return "重连中";
    if (wsReconnectFailed.value) return "连接失败";
    return "未连接到服务器";
  });

  function clearWsReconnectTimer() {
    window.clearTimeout(wsReconnectTimer);
    wsReconnectTimer = 0;
  }

  function connectWs(resetReconnect = false) {
    if (!token.value) {
      showMessage("请先登录");
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
      wsConnected.value = true;
      wsReconnecting.value = false;
      wsReconnectFailed.value = false;
      wsReconnectAttempts.value = 0;
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
      showMessage("实时通道连接失败，请点击状态点重试");
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
    conversations.value = [];
    friends.value = [];
    friendRequests.value = [];
    activeConversation.value = null;
    messages.value = [];
    activeRoomId.value = "";
    roomVideoHistory.value = [];
    danmakuItems.value = [];
    Object.assign(video, { fileId: "", url: "", objectKey: "", fileName: "" });
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
        const isActiveConversation = activeConversation.value?.conversationId === msg.conversationId;
        upsertConversationPreview(
          msg.conversationId,
          {
            latestMessage: {
              content: msg.content,
              convType: msg.convType,
              senderUsername: msg.senderUsername,
              sendTime: msg.sendTime || Date.now(),
            },
            unread: isActiveConversation ? 0 : undefined,
            unreadDelta: isActiveConversation ? 0 : 1,
            convType: msg.convType,
          },
          true,
        );
        if (isActiveConversation) {
          messages.value.push({
            messageId: msg.messageId,
            conversationId: msg.conversationId,
            senderId: msg.sendId,
            senderUsername: msg.senderUsername,
            content: msg.content,
            seq: msg.seq,
            convType: msg.convType,
            cType: msg.cType,
            sendTime: msg.sendTime || Date.now(),
          });
          scrollToBottom();
          sendReadAck(msg.conversationId, msg.seq);
        }
      }
      if (frame.op === "msg_ack") {
        const ack = decodePayload("messageAck", frame.data);
        if (ack.status === "failed") showMessage(ack.extra || "消息发送失败");
      }
      if (frame.op === "watch_video_sync") {
        applyWatchState(decodePayload("watchState", frame.data));
      }
    } catch (err) {
      showMessage(`消息解析失败：${err.message}`);
    }
  }

  function sendFrame(op, typeName, payload) {
    if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
      showMessage("WebSocket 未连接");
      return false;
    }
    ws.value.send(encodeFrame(op, typeName, payload));
    return true;
  }

  function sendMessage(options = {}) {
    const content = messageText.value.trim();
    if (!content || !activeConversation.value) return;
    const clientMsgId = crypto.randomUUID();
    const videoTime = videoRef.value ? Math.floor(videoRef.value.currentTime * 1000) : 0;
    const isWatchRoomMessage = Boolean(
      options.withVideoContext &&
      activeConversation.value.convType === 2 &&
        activeConversation.value.conversationId === activeRoomId.value &&
        video.fileId &&
        video.url,
    );
    const ok = sendFrame("msg", "messageReq", {
      clientMsgId,
      recvId: activeConversation.value.conversationId,
      convType: activeConversation.value.convType || 2,
      cType: 1,
      content,
      videoTime,
      hasVideoTime: isWatchRoomMessage,
    });
    if (!ok) return;
    messages.value.push({
      clientMsgId,
      senderId: currentUser.userId,
      senderUsername: currentUser.username,
      content,
      sendTime: Date.now(),
      convType: activeConversation.value.convType,
      videoId: isWatchRoomMessage ? video.fileId : "",
      videoTime: isWatchRoomMessage ? videoTime : null,
    });
    upsertConversationPreview(
      activeConversation.value.conversationId,
      {
        displayName: activeConversation.value.displayName,
        unread: 0,
        resetUnread: true,
        latestMessage: {
          content,
          convType: activeConversation.value.convType,
          senderUsername: currentUser.username,
          sendTime: Date.now(),
        },
        convType: activeConversation.value.convType,
      },
      true,
    );
    messageText.value = "";
    scrollToBottom();
  }

  async function handleCreateRoom() {
    try {
      const data = await createRoom(token.value, roomForm);
      activeRoomId.value = data.roomId;
      roomForm.roomName = data.roomName || roomForm.roomName;
      openConversation(data.roomId, 2, data.roomName || "一起看房间");
      roomForm.inviteCodeDisplay = data.inviteCode || "";
      loadRoomVideoHistory();
      showMessage("房间已创建", "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function handleJoinRoom() {
    try {
      const data = await joinRoom(token.value, roomForm.inviteCode);
      activeRoomId.value = data.roomId || "";
      roomForm.roomName = data.roomName || roomForm.roomName;
      if (activeRoomId.value) {
        openConversation(activeRoomId.value, 2, roomForm.roomName || "一起看房间");
        loadRoomVideoHistory();
      }
      showMessage("已加入房间", "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function handleInvite() {
    try {
      const data = await getInviteCode(token.value, activeRoomId.value);
      roomForm.inviteCodeDisplay = typeof data === "string" ? data : data.inviteCode;
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function handleUpload(event) {
    const file = event.target.files?.[0];
    if (!file) return;
    uploadName.value = file.name;
    try {
      const data = await chunkUpload.upload(token.value, file);
      Object.assign(video, { fileId: data.fileId, url: data.url, objectKey: data.objectKey, fileName: data.fileName });
      fileIdInput.value = data.fileId;
      loadDanmaku();
      showMessage("视频上传完成", "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function loadFile() {
    try {
      const data = await getFile(token.value, fileIdInput.value);
      Object.assign(video, { fileId: data.fileId, url: data.url, objectKey: data.objectKey, fileName: data.fileName });
      loadDanmaku();
      showMessage("视频已加载", "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  function loadVideoToRoom() {
    if (!activeRoomId.value) {
      showMessage("请先进入房间");
      return;
    }
    if (!video.url) {
      showMessage("请先加载视频");
      return;
    }
    sendWatchControl("load", { positionMs: 0 });
  }

  function sendWatchControl(action, patch = {}) {
    if (!activeRoomId.value) {
      showMessage("请先进入房间");
      return;
    }
    if (action !== "get_state" && !video.url) {
      showMessage("请先加载视频");
      return;
    }
    const current = videoRef.value ? Math.floor(videoRef.value.currentTime * 1000) : 0;
    sendFrame("watch_video_control", "watchControl", {
      roomId: activeRoomId.value,
      action,
      videoId: video.fileId,
      videoUrl: video.url,
      positionMs: current,
      durationMs: videoRef.value ? Math.floor((videoRef.value.duration || 0) * 1000) : 0,
      playbackRate: videoRef.value?.playbackRate || 1,
      clientTimeMs: Date.now(),
      ...patch,
    });
  }

  function seekBy(deltaMs) {
    if (videoRef.value) {
      applyingWatchState.value = true;
      videoRef.value.currentTime = Math.max(0, videoRef.value.currentTime + deltaMs / 1000);
      window.setTimeout(() => {
        applyingWatchState.value = false;
      }, 300);
    }
    sendWatchControl(deltaMs > 0 ? "forward" : "backward", { deltaMs: Math.abs(deltaMs) });
  }

  function applyWatchState(state) {
    if (state.roomId && state.roomId !== activeRoomId.value) return;
    applyingWatchState.value = true;
    const previousVideoId = video.fileId;
    if (state.action === "load" || state.videoUrl) video.fileId = state.videoId || "";
    if (state.videoUrl) video.url = state.videoUrl;
    if (video.fileId !== previousVideoId) loadDanmaku();
    nextTick(() => {
      if (!videoRef.value) {
        applyingWatchState.value = false;
        return;
      }
      const target = (state.positionMs || 0) / 1000;
      if (Math.abs(videoRef.value.currentTime - target) > 1.2) videoRef.value.currentTime = target;
      videoRef.value.playbackRate = state.playbackRate || 1;
      if (state.isPlaying) videoRef.value.play().catch(() => {});
      else videoRef.value.pause();
      window.setTimeout(() => {
        applyingWatchState.value = false;
      }, 300);
    });
  }

  async function loadDanmaku() {
    if (!activeRoomId.value || !video.fileId || !token.value) {
      danmakuItems.value = [];
      return;
    }
    const data = await getDanmaku(token.value, activeRoomId.value, video.fileId).catch(() => ({ items: [] }));
    danmakuItems.value = data.items || [];
  }

  async function loadRoomVideoHistory() {
    if (!activeRoomId.value || !token.value) {
      roomVideoHistory.value = [];
      return;
    }
    const data = await getRoomVideoHistory(token.value, activeRoomId.value).catch(() => ({ items: [] }));
    roomVideoHistory.value = Array.isArray(data.items) ? data.items : [];
  }

  async function selectRoomVideo(item) {
    if (!item?.videoId || !token.value) return;
    try {
      const data = await getFile(token.value, item.videoId);
      Object.assign(video, {
        fileId: data.fileId,
        url: data.url,
        objectKey: data.objectKey,
        fileName: data.fileName,
      });
      fileIdInput.value = data.fileId;
      await loadDanmaku();
      showMessage(`已切换到 ${data.fileName || data.fileId}`, "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  function onVideoTimeUpdate() {
    currentVideoTime.value = videoRef.value?.currentTime || 0;
  }

  function setVideoElement(el) {
    videoRef.value = el;
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
    clearWsReconnectTimer();
    wsManualClose = true;
    if (ws.value) {
      ws.value.onclose = null;
      ws.value.close();
    }
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
    wsReconnecting,
    wsReconnectFailed,
    wsStatusText,
    message,
    messageType,
    roomForm,
    activeRoomId,
    fileIdInput,
    uploadName,
    chunkUpload,
    video,
    applyingWatchState,
    videoRef,
    visibleDanmaku,
    roomVideoHistory,
    submitAuth,
    loadOffline,
    loadFriends,
    loadFriendRequests,
    selectConversation,
    openPrivateConversation,
    submitFriendRequest,
    handleFriendRequest,
    connectWs,
    retryWsConnection,
    logout,
    sendMessage,
    handleCreateRoom,
    handleJoinRoom,
    handleInvite,
    handleUpload,
    loadFile,
    loadVideoToRoom,
    loadRoomVideoHistory,
    selectRoomVideo,
    sendWatchControl,
    seekBy,
    onVideoTimeUpdate,
    setVideoElement,
    formatTime,
  };
}
