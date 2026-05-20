<template>
  <main class="shell">
    <aside class="rail">
      <div class="brand">
        <div class="brand-mark">IM</div>
        <div>
          <strong>IM Watch</strong>
          <span>一起看工作台</span>
        </div>
      </div>

      <section class="auth-panel">
        <div class="tabs">
          <button :class="{ active: authMode === 'login' }" @click="authMode = 'login'">登录</button>
          <button :class="{ active: authMode === 'register' }" @click="authMode = 'register'">注册</button>
        </div>
        <input v-model.trim="authForm.phone" placeholder="手机号" />
        <input v-model.trim="authForm.password" placeholder="密码" type="password" @keydown.enter="submitAuth" />
        <button class="primary wide" @click="submitAuth">
          <LogIn :size="16" />
          {{ authMode === "login" ? "进入" : "创建账号" }}
        </button>
        <p class="muted small" v-if="currentUser.userId">当前用户：{{ currentUser.userId }}</p>
      </section>

      <section class="status-card">
        <div class="status-line">
          <span :class="['dot', wsConnected ? 'ok' : '']"></span>
          <span>{{ wsConnected ? "WebSocket 已连接" : "WebSocket 未连接" }}</span>
        </div>
        <button class="ghost wide" :disabled="!token" @click="connectWs">
          <RadioTower :size="16" />
          连接实时通道
        </button>
      </section>
    </aside>

    <section class="conversation-pane">
      <div class="pane-head">
        <div>
          <p class="eyebrow">Conversations</p>
          <h1>消息</h1>
        </div>
        <button class="icon-btn" :disabled="!token" title="刷新会话" @click="loadOffline">
          <RefreshCcw :size="18" />
        </button>
      </div>

      <div class="quick-form">
        <input v-model.trim="manualConversationId" placeholder="会话 / 房间 ID" />
        <select v-model.number="manualConvType">
          <option :value="1">私聊</option>
          <option :value="2">群聊</option>
        </select>
        <button class="ghost" @click="openManualConversation">打开</button>
      </div>

      <div class="conversation-list">
        <button
          v-for="item in conversations"
          :key="item.conversationId"
          :class="['conversation-item', activeConversation?.conversationId === item.conversationId ? 'active' : '']"
          @click="selectConversation(item)"
        >
          <div class="avatar">{{ item.conversationId.slice(-2).toUpperCase() }}</div>
          <div class="item-main">
            <div class="item-row">
              <strong>{{ item.conversationId }}</strong>
              <span v-if="item.unread" class="badge">{{ item.unread }}</span>
            </div>
            <p>{{ item.latestMessage?.content || "暂无消息" }}</p>
          </div>
        </button>
      </div>
    </section>

    <section class="chat-pane">
      <header class="chat-head">
        <div>
          <p class="eyebrow">Chat Room</p>
          <h2>{{ activeConversation?.conversationId || "选择或打开一个会话" }}</h2>
        </div>
        <span class="pill">{{ activeConversation?.convType === 2 ? "群聊" : "私聊" }}</span>
      </header>

      <div class="messages" ref="messageList">
        <div v-if="!activeConversation" class="empty">
          <MessageCircle :size="42" />
          <p>登录后刷新会话，或手动输入用户 ID / 房间 ID。</p>
        </div>
        <article
          v-for="msg in messages"
          :key="msg.clientMsgId || msg.messageId"
          :class="['bubble-row', msg.senderId === currentUser.userId ? 'mine' : '']"
        >
          <div class="bubble">
            <div class="bubble-meta">
              <span>{{ msg.senderId || "me" }}</span>
              <time>{{ formatTime(msg.sendTime) }}</time>
            </div>
            <p>{{ msg.content }}</p>
          </div>
        </article>
      </div>

      <footer class="composer">
        <input
          v-model="messageText"
          :disabled="!activeConversation || !wsConnected"
          placeholder="输入消息"
          @keydown.enter="sendMessage"
        />
        <button class="primary" :disabled="!activeConversation || !wsConnected" @click="sendMessage">
          <Send :size="16" />
          发送
        </button>
      </footer>
    </section>

    <aside class="watch-pane">
      <section class="watch-card video-card">
        <div class="pane-head compact">
          <div>
            <p class="eyebrow">Watch Together</p>
            <h2>一起看</h2>
          </div>
          <button class="icon-btn" :disabled="!activeRoomId" title="同步状态" @click="sendWatchControl('get_state')">
            <RotateCw :size="18" />
          </button>
        </div>

        <div class="video-wrap">
          <video ref="videoRef" :src="video.url" controls @timeupdate="onVideoTimeUpdate"></video>
          <div class="danmaku-layer">
            <span
              v-for="item in visibleDanmaku"
              :key="item.messageId"
              class="danmaku"
              :style="{ top: `${item.top}%`, animationDuration: `${item.duration}s` }"
            >
              {{ item.content }}
            </span>
          </div>
        </div>

        <div class="watch-controls">
          <button class="icon-btn" :disabled="!activeRoomId" title="后退 10 秒" @click="seekBy(-10000)">
            <SkipBack :size="18" />
          </button>
          <button class="icon-btn strong" :disabled="!activeRoomId" title="播放" @click="sendWatchControl('play')">
            <Play :size="18" />
          </button>
          <button class="icon-btn" :disabled="!activeRoomId" title="暂停" @click="sendWatchControl('pause')">
            <Pause :size="18" />
          </button>
          <button class="icon-btn" :disabled="!activeRoomId" title="前进 10 秒" @click="seekBy(10000)">
            <SkipForward :size="18" />
          </button>
        </div>
      </section>

      <section class="watch-card">
        <h3>房间</h3>
        <input v-model.trim="roomForm.roomName" placeholder="房间名称" />
        <div class="split">
          <button class="ghost" :disabled="!token" @click="handleCreateRoom">创建</button>
          <button class="ghost" :disabled="!activeRoomId" @click="handleInvite">邀请码</button>
        </div>
        <div class="split">
          <input v-model.trim="roomForm.inviteCode" placeholder="输入邀请码" />
          <button class="ghost" :disabled="!token" @click="handleJoinRoom">加入</button>
        </div>
        <p class="muted small">当前房间：{{ activeRoomId || "未选择" }}</p>
        <p v-if="roomForm.inviteCodeDisplay" class="code-box">{{ roomForm.inviteCodeDisplay }}</p>
      </section>

      <section class="watch-card">
        <h3>视频</h3>
        <label class="file-box">
          <Upload :size="18" />
          <span>{{ uploadName || "上传视频文件" }}</span>
          <input type="file" accept="video/*" @change="handleUpload" />
        </label>
        <div class="split">
          <input v-model.trim="fileIdInput" placeholder="fileId" />
          <button class="ghost" :disabled="!fileIdInput" @click="loadFile">加载</button>
        </div>
        <input v-model.trim="video.url" placeholder="视频 URL" />
        <button class="primary wide" :disabled="!activeRoomId || !video.url" @click="loadVideoToRoom">
          <Film :size="16" />
          同步到房间
        </button>
      </section>
    </aside>

    <div v-if="toast" class="toast">{{ toast }}</div>
  </main>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, reactive, ref } from "vue";
import {
  createRoom,
  getDanmaku,
  getFile,
  getHistoryMessages,
  getInviteCode,
  getOfflineMessages,
  joinRoom,
  login,
  register,
  uploadFile,
} from "./api";
import { decodeFrame, decodePayload, encodeFrame } from "./wsProto";
import {
  Film,
  LogIn,
  MessageCircle,
  Pause,
  Play,
  RadioTower,
  RefreshCcw,
  RotateCw,
  Send,
  SkipBack,
  SkipForward,
  Upload,
} from "@lucide/vue";

const token = ref(localStorage.getItem("im_token") || "");
const currentUser = reactive(JSON.parse(localStorage.getItem("im_user") || "{}"));
const authMode = ref("login");
const authForm = reactive({ phone: "", password: "" });
const conversations = ref([]);
const activeConversation = ref(null);
const messages = ref([]);
const messageText = ref("");
const manualConversationId = ref("");
const manualConvType = ref(2);
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
    await loadOffline();
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

function openManualConversation() {
  if (!manualConversationId.value) return;
  const item = {
    conversationId: manualConversationId.value,
    unread: 0,
    latestMessage: { content: "手动打开", convType: manualConvType.value },
    convType: manualConvType.value,
  };
  conversations.value = [item, ...conversations.value.filter((v) => v.conversationId !== item.conversationId)];
  selectConversation(item);
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
    manualConversationId.value = data.roomId;
    manualConvType.value = 2;
    openManualConversation();
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
      manualConversationId.value = activeRoomId.value;
      manualConvType.value = 2;
      openManualConversation();
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
</script>
