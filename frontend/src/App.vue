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
        <button :class="{ active: directoryMode === 'conversations' }" title="会话" @click="showConversations">
          <MessageCircle :size="22" />
        </button>
        <button :class="{ active: directoryMode === 'friends' }" title="好友" @click="showFriends">
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

    <section class="chat-workspace" :style="{ gridTemplateColumns: `${chatSidebarWidth}px minmax(0, 1fr)` }">
      <aside class="chat-directory">
        <div class="chat-resize-handle" @pointerdown="startChatSidebarResize"></div>

        <ConversationList v-if="directoryMode === 'conversations'" key="conversations" :token="token"
          :conversations="conversations" :active-conversation="activeConversation" @load-offline="refreshConversations"
          :refreshing="refreshingOffline" @select-conversation="selectConversation" />

        <FriendsPanel v-else key="friends" :token="token" :friends="friends" :friend-requests="friendRequests"
          :friend-form="friendForm" :format-time="formatTime" @refresh="refreshFriends" :refreshing="refreshingFriends"
          @submit-request="submitFriendRequest" @operate-request="handleFriendRequest"
          @open-chat="openPrivateConversation" />
      </aside>

      <ChatPanel v-model:message-text="messageText" :active-conversation="activeConversation" :messages="messages"
        :conversation-loading="conversationLoading" :current-user="currentUser" :ws-connected="wsConnected"
        :set-message-list-ref="setMessageListRef" :last-read-seq="activeReadState.lastReadSeq"
        :read-receivers="activeReadState.readers" @send-message="sendMessage" @send-image="sendImageMessage"
        @send-file="sendFileMessage" @send-video="sendVideoMessage" />
    </section>
  </main>

  <div v-if="message" :class="['message-bar', messageType]">
    <span class="message-marker" aria-hidden="true"></span>
    <span class="message-content">{{ message }}</span>
  </div>
</template>

<script setup>
import { LogOut, MessageCircle, MoonStar, SunMedium, Users } from "@lucide/vue";
import { computed, onBeforeUnmount, ref, watch } from "vue";
import ChatPanel from "./components/ChatPanel.vue";
import ConversationList from "./components/ConversationList.vue";
import FriendsPanel from "./components/FriendsPanel.vue";
import LoginPage from "./components/LoginPage.vue";
import { useImClient } from "./composables/useImClient";

const themeMode = ref(resolveInitialTheme());
const directoryMode = ref("conversations");
const chatSidebarWidth = ref(330);
const chatSidebarMinWidth = 280;
const chatSidebarMaxWidth = 520;
let chatSidebarResizeStartX = 0;
let chatSidebarResizeStartWidth = 0;
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
  activeReadState,
  conversationLoading,
  messageText,
  messageList,
  wsConnected,
  wsReconnecting,
  wsReconnectFailed,
  wsStatusText,
  message,
  messageType,
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
  sendImageMessage,
  sendFileMessage,
  sendVideoMessage,
  formatTime,
} = useImClient();

const setMessageListRef = (el) => {
  messageList.value = el;
};

const themeLabel = computed(() => (themeMode.value === "dark" ? "切换白天模式" : "切换黑夜模式"));

watch(
  themeMode,
  (mode) => {
    const root = document.documentElement;
    root.dataset.theme = mode;
    root.style.colorScheme = mode === "dark" ? "dark" : "light";
    // 保存用户风格
    localStorage.setItem("im_theme", mode);
  },
  { immediate: true },
);

function toggleTheme() {
  themeMode.value = themeMode.value === "dark" ? "light" : "dark";
}

function showConversations() {
  directoryMode.value = "conversations";
}

function showFriends() {
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

function setSidebarResizing(next) {
  if (typeof document === "undefined") return;
  document.body.classList.toggle("sidebar-resizing", next);
}

onBeforeUnmount(() => {
  window.removeEventListener("pointermove", resizeChatSidebar);
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

  .chat-workspace {
    grid-template-columns: 1fr;
  }

  .chat-directory {
    border-right: 0;
    border-bottom: 1px solid var(--border);
  }
}
</style>
