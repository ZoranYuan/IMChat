<template>
  <LoginPage v-if="!token" v-model:auth-mode="authMode" :auth-form="authForm" :theme-mode="themeMode"
    :theme-label="themeLabel" @submit-auth="submitAuth" @oauth-login="handleOAuthLogin" @toggle-theme="toggleTheme" />

  <main v-else class="app-shell">
    <aside class="app-menu">
      <div class="menu-user">
        <div class="brand-mark">SY</div>
        <span>{{ currentUser.username || "用户" }}</span>
      </div>

      <nav class="menu-nav">
        <button :class="{ active: viewMode === 'chat' && directoryMode === 'conversations' }" title="会话"
          @click="showConversations">
          <MessageCircle :size="22" />
        </button>
        <button :class="{ active: viewMode === 'chat' && directoryMode === 'friends' }" title="好友" @click="showFriends">
          <Users :size="22" />
        </button>
      </nav>

      <div class="menu-actions">
        <button class="menu-status" :title="wsStatusText" @click="retryWsConnection">
          <span :class="['dot', { ok: wsConnected, warn: wsReconnecting, fail: wsReconnectFailed }]"></span>
        </button>
        <button class="menu-icon" title="退出登录" @click="logout">
          <LogOut :size="20" />
        </button>
      </div>

      <button class="theme-toggle" type="button" :title="themeLabel" @click="toggleTheme">
        <SunMedium v-if="themeMode === 'dark'" :size="16" />
        <MoonStar v-else :size="16" />
      </button>
    </aside>

    <section v-if="viewMode === 'chat'" key="chat-workspace" class="chat-workspace"
      :style="{ gridTemplateColumns: `${chatSidebarWidth}px minmax(0, 1fr)` }">
      <aside class="chat-directory">
        <div class="chat-resize-handle" @pointerdown="startChatSidebarResize"></div>

        <ConversationList v-if="directoryMode === 'conversations'" key="conversations" :token="token"
          :conversations="conversations" :active-conversation="activeConversation" @load-offline="refreshConversations"
          :refreshing="refreshingOffline" @select-conversation="selectConversation" />

        <FriendsPanel v-else key="friends" :token="token" :friends="friends" :friend-requests="friendRequests"
          :friend-form="friendForm" :format-time="formatTime" @refresh="refreshFriends"
          :refreshing="refreshingFriends"
          @submit-request="submitFriendRequest" @operate-request="handleFriendRequest"
          @open-chat="openPrivateConversation" />
      </aside>

      <ChatPanel v-model:message-text="messageText" :active-conversation="activeConversation" :messages="messages"
        :conversation-loading="conversationLoading" :current-user="currentUser" :ws-connected="wsConnected" :set-message-list-ref="setMessageListRef"
        :show-watch-entry="Boolean(activeConversation?.convType === 2)" :watch-entry-label="watchActionLabel"
        :watch-entry-hint="watchStatusLabel" @send-message="sendMessage" @open-watch="openWatchRoom" />
    </section>

    <section v-else key="watch-workspace" :class="['watch-workspace', { 'chat-collapsed': watchChatCollapsed }]"
      :style="{ gridTemplateColumns: watchChatCollapsed ? '44px minmax(0, 1fr)' : `${watchSidebarWidth}px minmax(0, 1fr)` }">
      <aside class="watch-chat-sidebar">
        <button class="collapse-tab" @click="watchChatCollapsed = !watchChatCollapsed">
          <PanelLeftClose v-if="!watchChatCollapsed" :size="18" />
          <PanelLeftOpen v-else :size="18" />
        </button>

        <div v-if="!watchChatCollapsed" class="watch-resize-handle" @pointerdown="startWatchSidebarResize"></div>

        <template v-if="!watchChatCollapsed">
          <div class="watch-room-chat-head">
            <span>房间聊天</span>
            <strong>{{ activeRoomName || "未进入房间" }}</strong>
          </div>

          <ChatPanel v-model:message-text="messageText" :active-conversation="watchConversation"
            :messages="watchMessages" :conversation-loading="false" :current-user="currentUser" :ws-connected="wsConnected"
            :set-message-list-ref="setMessageListRef" :show-watch-entry="true" :watch-entry-label="watchActionLabel"
            :watch-entry-hint="watchStatusLabel" @send-message="sendWatchMessage" @open-watch="openWatchRoom" />
        </template>
      </aside>

        <WatchPanel v-model:file-id-input="fileIdInput" :token="token" :active-room-id="activeRoomId"
        :active-room-name="activeRoomName" :can-control-video="canControlWatchVideo" :room-form="roomForm"
        :upload-name="uploadName" :chunk-upload="chunkUpload" :video="video"
        :visible-danmaku="visibleDanmaku" :watch-session="watchSession" :watch-action-label="watchActionLabel"
        :watch-status-label="watchStatusLabel" :watch-owner-label="watchOwnerLabel"
        :can-start-watch-session="canStartWatchSession" :can-stop-watch-session="canStopWatchSession"
        :suppress-native-controls="applyingWatchState"
        @update:video-el="setVideoElement"
        @watch-control="sendWatchControl" @seek-by="seekBy" @video-time-update="onVideoTimeUpdate"
        @create-room="handleCreateRoom" @join-room="handleJoinRoom" @invite="handleInvite" @upload="handleUpload"
        @load-file="loadFile" @load-video="loadVideoToRoom" @stop-watch="stopWatchSession"
        @native-video-control="handleNativeVideoControl" />
    </section>
  </main>

  <div v-if="message" :class="['message-bar', messageType]">
    <span class="message-marker" aria-hidden="true"></span>
    <span class="message-content">{{ message }}</span>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import ChatPanel from "./components/ChatPanel.vue";
import ConversationList from "./components/ConversationList.vue";
import FriendsPanel from "./components/FriendsPanel.vue";
import LoginPage from "./components/LoginPage.vue";
import WatchPanel from "./components/WatchPanel.vue";
import { useImClient } from "./composables/useImClient";
import { LogOut, MessageCircle, MoonStar, PanelLeftClose, PanelLeftOpen, SunMedium, Users } from "@lucide/vue";

const viewMode = ref("chat");
const themeMode = ref(resolveInitialTheme());
const directoryMode = ref("conversations");
const chatSidebarWidth = ref(330);
const chatSidebarMinWidth = 280;
const chatSidebarMaxWidth = 520;
let chatSidebarResizeStartX = 0;
let chatSidebarResizeStartWidth = 0;
const watchChatCollapsed = ref(false);
const watchSidebarWidth = ref(430);
const watchSidebarMinWidth = 320;
const watchSidebarMaxWidth = 560;
let watchSidebarResizeStartX = 0;
let watchSidebarResizeStartWidth = 0;
let oauthToastTimer = 0;
const refreshingOffline = ref(false);
const refreshingFriends = ref(false);
let refreshToastTimer = 0;

const {
  token,
  currentUser,
  authMode,
  authForm,
  conversations,
  friends,
  friendRequests,
  friendForm,
  activeConversation,
  messages,
  conversationLoading,
  messageText,
  messageList,
  wsConnected,
  wsReconnecting,
  wsReconnectFailed,
  wsStatusText,
  message,
  messageType,
  showMessage,
  roomForm,
  activeRoomId,
  fileIdInput,
  uploadName,
  chunkUpload,
  video,
  watchSession,
  canControlWatchVideo,
  canStartWatchSession,
  canStopWatchSession,
  applyingWatchState,
  visibleDanmaku,
  watchActionLabel,
  watchStatusLabel,
  watchOwnerLabel,
  submitAuth,
  loadOffline,
  loadFriends,
  loadFriendRequests,
  selectConversation,
  openPrivateConversation,
  submitFriendRequest,
  handleFriendRequest,
  retryWsConnection,
  logout,
  sendMessage,
  handleCreateRoom,
  handleJoinRoom,
  handleInvite,
  handleUpload,
  loadFile,
  loadVideoToRoom,
  sendWatchControl,
  stopWatchSession,
  seekBy,
  onVideoTimeUpdate,
  setVideoElement,
  formatTime,
} = useImClient();

const setMessageListRef = (el) => {
  messageList.value = el;
};

const themeLabel = computed(() => (themeMode.value === "dark" ? "切换白天模式" : "切换黑夜模式"));

const activeRoomName = computed(() => {
  if (!activeRoomId.value) return "";
  const roomConversation = conversations.value.find((item) => item.conversationId === activeRoomId.value);
  return roomConversation?.displayName || roomForm.roomName || "一起看房间";
});

const watchConversation = computed(() => {
  if (!activeRoomId.value) return null;
  return (
    conversations.value.find((item) => item.conversationId === activeRoomId.value) || {
      conversationId: activeRoomId.value,
      displayName: activeRoomName.value,
      convType: 2,
    }
  );
});

const watchMessages = computed(() => {
  if (activeConversation.value?.conversationId !== activeRoomId.value) return [];
  return messages.value;
});

function focusWatchRoomConversation() {
  if (!activeRoomId.value) return Promise.resolve();
  if (activeConversation.value?.conversationId === activeRoomId.value) return Promise.resolve();
  return selectConversation(watchConversation.value);
}

watch(viewMode, (mode) => {
  if (mode === "watch") focusWatchRoomConversation();
});

watch(
  themeMode,
  (mode) => {
    const root = document.documentElement;
    root.dataset.theme = mode;
    root.style.colorScheme = mode === "dark" ? "dark" : "light";
    localStorage.setItem("im_theme", mode);
  },
  { immediate: true },
);

async function sendWatchMessage() {
  await focusWatchRoomConversation();
  if (activeConversation.value?.conversationId !== activeRoomId.value) return;
  sendMessage({ withVideoContext: true });
}

function openWatchRoom() {
  if (!activeConversation.value || activeConversation.value.convType !== 2) {
    showMessage("只有群聊房间可以一起看");
    return;
  }
  viewMode.value = "watch";
  focusWatchRoomConversation();
}

function handleNativeVideoControl(action) {
  sendWatchControl(action);
}

function toggleTheme() {
  themeMode.value = themeMode.value === "dark" ? "light" : "dark";
}

function showConversations() {
  viewMode.value = "chat";
  directoryMode.value = "conversations";
}

function showFriends() {
  viewMode.value = "chat";
  directoryMode.value = "friends";
}

function startChatSidebarResize(event) {
  event.preventDefault();
  setSidebarResizing(true);
  chatSidebarResizeStartX = event.clientX;
  chatSidebarResizeStartWidth = chatSidebarWidth.value;
  window.addEventListener("pointermove", resizeChatSidebar);
  window.addEventListener("pointerup", stopChatSidebarResize, { once: true });
}

function resizeChatSidebar(event) {
  const nextWidth = chatSidebarResizeStartWidth + event.clientX - chatSidebarResizeStartX;
  chatSidebarWidth.value = Math.min(chatSidebarMaxWidth, Math.max(chatSidebarMinWidth, nextWidth));
}

function stopChatSidebarResize() {
  window.removeEventListener("pointermove", resizeChatSidebar);
  setSidebarResizing(false);
}

function startWatchSidebarResize(event) {
  if (watchChatCollapsed.value) return;
  event.preventDefault();
  setSidebarResizing(true);
  watchSidebarResizeStartX = event.clientX;
  watchSidebarResizeStartWidth = watchSidebarWidth.value;
  window.addEventListener("pointermove", resizeWatchSidebar);
  window.addEventListener("pointerup", stopWatchSidebarResize, { once: true });
}

function resizeWatchSidebar(event) {
  const nextWidth = watchSidebarResizeStartWidth + event.clientX - watchSidebarResizeStartX;
  watchSidebarWidth.value = Math.min(watchSidebarMaxWidth, Math.max(watchSidebarMinWidth, nextWidth));
}

function stopWatchSidebarResize() {
  window.removeEventListener("pointermove", resizeWatchSidebar);
  setSidebarResizing(false);
}

function setSidebarResizing(next) {
  if (typeof document === "undefined") return;
  document.body.classList.toggle("sidebar-resizing", next);
}

onBeforeUnmount(() => {
  window.removeEventListener("pointermove", resizeChatSidebar);
  window.removeEventListener("pointermove", resizeWatchSidebar);
  setSidebarResizing(false);
});

function handleOAuthLogin(provider) {
  const providerName = provider === "wechat" ? "微信" : "GitHub";
  message.value = `${providerName} 登录后端接口待接入`;
  messageType.value = "warning";
  window.clearTimeout(oauthToastTimer);
  oauthToastTimer = window.setTimeout(() => {
    message.value = "";
    messageType.value = "info";
  }, 2400);
}

function showAppMessage(nextMessage, type = "success") {
  message.value = nextMessage;
  messageType.value = type;
  window.clearTimeout(refreshToastTimer);
  refreshToastTimer = window.setTimeout(() => {
    message.value = "";
    messageType.value = "info";
  }, 2400);
}

async function refreshConversations() {
  if (refreshingOffline.value) return;
  refreshingOffline.value = true;
  try {
    const ok = await loadOffline();
    if (ok) showAppMessage("会话已刷新", "success");
  } finally {
    refreshingOffline.value = false;
  }
}

async function refreshFriends() {
  if (refreshingFriends.value) return;
  refreshingFriends.value = true;
  try {
    const [friendsOk, requestsOk] = await Promise.all([loadFriends(), loadFriendRequests()]);
    if (friendsOk && requestsOk) showAppMessage("好友已刷新", "success");
  } finally {
    refreshingFriends.value = false;
  }
}

function resolveInitialTheme() {
  if (typeof window === "undefined") return "dark";
  const stored = window.localStorage.getItem("im_theme");
  if (stored === "light" || stored === "dark") return stored;
  return window.matchMedia?.("(prefers-color-scheme: light)").matches ? "light" : "dark";
}
</script>

<style scoped>
:global(body.sidebar-resizing) {
  user-select: none;
  cursor: col-resize;
}

:global(body.sidebar-resizing *) {
  user-select: none;
}

.theme-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  margin-top: auto;
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 0;
  background: var(--surface-strong);
  color: var(--text);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(18px);
}

.app-shell {
  display: grid;
  grid-template-columns: 92px minmax(0, 1fr);
  min-height: 100vh;
  overflow: hidden;
}

.app-menu {
  display: grid;
  grid-template-rows: auto 1fr auto auto;
  justify-items: center;
  gap: 18px;
  padding: 16px 14px;
  border-right: 1px solid var(--border);
  background: rgba(10, 9, 18, 0.72);
  backdrop-filter: blur(22px);
}

.menu-user {
  display: grid;
  justify-items: center;
  gap: 10px;
  width: 100%;
  color: var(--muted);
  font-size: 12px;
}

.menu-user span {
  overflow: hidden;
  max-width: 60px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-menu .brand-mark {
  width: 40px;
  height: 40px;
  border-radius: 14px;
  font-size: 13px;
}

.menu-nav,
.menu-actions {
  display: grid;
  gap: 12px;
  justify-items: center;
  width: 100%;
}

.menu-nav {
  align-self: start;
  padding-top: 10px;
}

.menu-actions {
  align-self: end;
}

.menu-nav button.active {
  box-shadow: 0 16px 32px rgba(141, 91, 255, 0.28);
}

.menu-status {
  position: relative;
}

.chat-workspace {
  display: grid;
  grid-template-columns: minmax(280px, 340px) minmax(0, 1fr);
  min-height: 0;
  overflow: hidden;
}

.chat-directory {
  position: relative;
  min-width: 0;
  min-height: 0;
  border-right: 1px solid var(--border);
  background: rgba(18, 14, 31, 0.72);
  backdrop-filter: blur(18px);
}

.chat-resize-handle {
  position: absolute;
  top: 0;
  right: -4px;
  z-index: 8;
  width: 8px;
  height: 100%;
  cursor: col-resize;
}

.chat-resize-handle::after {
  content: "";
  position: absolute;
  top: 0;
  left: 3px;
  width: 1px;
  height: 100%;
  background: rgba(141, 91, 255, 0.28);
}

.watch-workspace {
  position: relative;
  display: grid;
  grid-template-columns: minmax(320px, 440px) minmax(0, 1fr);
  min-height: 0;
  overflow: hidden;
}

.watch-workspace.chat-collapsed {
  grid-template-columns: 44px minmax(0, 1fr);
}

.watch-chat-sidebar {
  position: relative;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  min-width: 0;
  min-height: 0;
  border-right: 1px solid var(--border);
  background: rgba(16, 13, 27, 0.74);
  backdrop-filter: blur(18px);
}

.watch-workspace.chat-collapsed .watch-chat-sidebar {
  display: block;
  overflow: hidden;
}

.watch-resize-handle {
  position: absolute;
  top: 0;
  right: -4px;
  z-index: 8;
  width: 8px;
  height: 100%;
  cursor: col-resize;
}

.watch-resize-handle::after {
  content: "";
  position: absolute;
  top: 0;
  left: 3px;
  width: 1px;
  height: 100%;
  background: rgba(141, 91, 255, 0.28);
}

.collapse-tab {
  position: absolute;
  top: 12px;
  right: 10px;
  z-index: 6;
  background: var(--surface-strong);
}

.watch-workspace.chat-collapsed .collapse-tab {
  left: 1px;
  right: auto;
  top: 16px;
}

.watch-room-chat-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
  color: var(--muted);
  font-size: 12px;
}

.watch-room-chat-head strong {
  overflow: hidden;
  color: var(--text);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.message-bar {
  position: fixed;
  top: 20px;
  left: 50%;
  z-index: 50;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  max-width: min(520px, calc(100vw - 32px));
  border: 1px solid var(--border);
  border-radius: 16px;
  padding: 12px 16px;
  background: var(--surface-strong);
  color: var(--text);
  box-shadow: var(--shadow-lg);
  transform: translateX(-50%);
  backdrop-filter: blur(18px);
}

.message-marker {
  flex: 0 0 auto;
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--muted);
  box-shadow: 0 0 0 4px rgba(255, 255, 255, 0.04);
}

.message-content {
  min-width: 0;
}

.message-bar.success {
  border-color: rgba(46, 229, 157, 0.28);
  background: rgba(46, 229, 157, 0.16);
}

.message-bar.success .message-marker {
  background: var(--success);
  box-shadow: 0 0 0 4px rgba(46, 229, 157, 0.16);
}

.message-bar.warning {
  border-color: rgba(240, 200, 90, 0.28);
  background: rgba(240, 200, 90, 0.16);
}

.message-bar.warning .message-marker {
  background: var(--warning);
  box-shadow: 0 0 0 4px rgba(240, 200, 90, 0.16);
}

.message-bar.danger {
  border-color: rgba(255, 95, 122, 0.28);
  background: rgba(255, 95, 122, 0.16);
}

.message-bar.danger .message-marker {
  background: var(--danger);
  box-shadow: 0 0 0 4px rgba(255, 95, 122, 0.16);
}

.message-bar.info {
  border-color: var(--border);
  background: var(--surface-strong);
}

.message-bar.info .message-marker {
  background: var(--primary);
  box-shadow: 0 0 0 4px rgba(141, 91, 255, 0.16);
}

@media (max-width: 1180px) {
  .theme-toggle {
    top: 12px;
    right: 12px;
  }

  .app-shell {
    grid-template-columns: 1fr;
  }

  .app-menu {
    grid-template-columns: auto 1fr auto;
    grid-template-rows: 1fr;
    align-items: center;
    justify-items: stretch;
    min-height: 76px;
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }

  .menu-user {
    justify-self: start;
    justify-items: start;
    width: auto;
  }

  .menu-nav {
    grid-auto-flow: column;
    align-self: center;
    justify-self: center;
    padding-top: 0;
  }

  .menu-actions {
    grid-auto-flow: column;
    align-self: center;
    justify-self: end;
  }

  .chat-workspace,
  .watch-workspace {
    grid-template-columns: 1fr;
  }

  .chat-directory,
  .watch-chat-sidebar {
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }
}
</style>
