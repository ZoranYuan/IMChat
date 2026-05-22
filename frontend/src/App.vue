<template>
  <LoginPage v-if="!token" v-model:auth-mode="authMode" :auth-form="authForm" @submit-auth="submitAuth"
    @oauth-login="handleOAuthLogin" />

  <main v-else class="app-shell">
    <aside class="app-menu">
      <div class="menu-user">
        <div class="brand-mark">SY</div>
        <span>{{ currentUser.username || "用户" }}</span>
      </div>

      <nav class="menu-nav">
        <button
          :class="{ active: viewMode === 'chat' && directoryMode === 'conversations' }"
          title="会话"
          @click="showConversations"
        >
          <MessageCircle :size="22" />
        </button>
        <button
          :class="{ active: viewMode === 'chat' && directoryMode === 'friends' }"
          title="好友"
          @click="showFriends"
        >
          <Users :size="22" />
        </button>
        <button :class="{ active: viewMode === 'watch' }" title="一起看" @click="viewMode = 'watch'">
          <Film :size="22" />
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
    </aside>

    <section v-if="viewMode === 'chat'" class="chat-workspace" :style="{ gridTemplateColumns: `${chatSidebarWidth}px minmax(0, 1fr)` }">
      <aside class="chat-directory">
        <div class="chat-resize-handle" @pointerdown="startChatSidebarResize"></div>

        <ConversationList v-if="directoryMode === 'conversations'" :token="token" :conversations="conversations"
          :active-conversation="activeConversation" @load-offline="loadOffline"
          @select-conversation="selectConversation" />

        <FriendsPanel v-else :token="token" :friends="friends" :friend-requests="friendRequests"
          :friend-form="friendForm" :format-time="formatTime" @refresh="refreshFriends"
          @submit-request="submitFriendRequest" @operate-request="handleFriendRequest"
          @open-chat="openPrivateConversation" />
      </aside>

      <ChatPanel v-model:message-text="messageText" :active-conversation="activeConversation" :messages="messages"
        :current-user="currentUser" :ws-connected="wsConnected" :message-list-ref="messageList"
        @send-message="sendMessage" />
    </section>

    <section v-else :class="['watch-workspace', { 'chat-collapsed': watchChatCollapsed }]"
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

          <ChatPanel v-model:message-text="messageText" :active-conversation="watchConversation" :messages="watchMessages"
            :current-user="currentUser" :ws-connected="wsConnected" :message-list-ref="messageList"
            @send-message="sendWatchMessage" />
        </template>
      </aside>

      <WatchPanel v-model:file-id-input="fileIdInput" :token="token" :active-room-id="activeRoomId"
        :active-room-name="activeRoomName" :can-control-video="canControlWatchVideo" :room-form="roomForm"
        :upload-name="uploadName" :chunk-upload="chunkUpload" :video="video" :room-video-history="roomVideoHistory"
        :visible-danmaku="visibleDanmaku"
        :suppress-native-controls="applyingWatchState"
        @update:video-el="setVideoElement" @watch-control="sendWatchControl" @seek-by="seekBy"
        @video-time-update="onVideoTimeUpdate" @create-room="handleCreateRoom" @join-room="handleJoinRoom"
        @invite="handleInvite" @upload="handleUpload" @load-file="loadFile" @load-video="loadVideoToRoom"
        @select-history-video="selectRoomVideo" @refresh-history="loadRoomVideoHistory"
        @native-video-control="handleNativeVideoControl" />
    </section>
  </main>

  <div v-if="toast" class="toast">{{ toast }}</div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import ChatPanel from "./components/ChatPanel.vue";
import ConversationList from "./components/ConversationList.vue";
import FriendsPanel from "./components/FriendsPanel.vue";
import LoginPage from "./components/LoginPage.vue";
import WatchPanel from "./components/WatchPanel.vue";
import { useImClient } from "./composables/useImClient";
import { Film, LogOut, MessageCircle, PanelLeftClose, PanelLeftOpen, Users } from "@lucide/vue";

const viewMode = ref("chat");
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
  messageText,
  messageList,
  wsConnected,
  wsReconnecting,
  wsReconnectFailed,
  wsStatusText,
  toast,
  roomForm,
  activeRoomId,
  fileIdInput,
  uploadName,
  chunkUpload,
  video,
  applyingWatchState,
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
} = useImClient();

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

const canControlWatchVideo = computed(() => Boolean(activeRoomId.value && video.url));

function focusWatchRoomConversation() {
  if (!activeRoomId.value) return Promise.resolve();
  if (activeConversation.value?.conversationId === activeRoomId.value) return Promise.resolve();
  return selectConversation(watchConversation.value);
}

watch(viewMode, (mode) => {
  if (mode === "watch") focusWatchRoomConversation();
});

async function sendWatchMessage() {
  await focusWatchRoomConversation();
  if (activeConversation.value?.conversationId !== activeRoomId.value) return;
  sendMessage({ withVideoContext: true });
}

function handleNativeVideoControl(action) {
  sendWatchControl(action);
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
}

function startWatchSidebarResize(event) {
  if (watchChatCollapsed.value) return;
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
}

onBeforeUnmount(() => {
  window.removeEventListener("pointermove", resizeChatSidebar);
  window.removeEventListener("pointermove", resizeWatchSidebar);
});

function handleOAuthLogin(provider) {
  const providerName = provider === "wechat" ? "微信" : "GitHub";
  toast.value = `${providerName} 登录后端接口待接入`;
  window.clearTimeout(oauthToastTimer);
  oauthToastTimer = window.setTimeout(() => {
    toast.value = "";
  }, 2400);
}

function refreshFriends() {
  loadFriends();
  loadFriendRequests();
}
</script>
