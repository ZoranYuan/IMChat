<template>
  <LoginPage
    v-if="!token"
    v-model:auth-mode="authMode"
    :auth-form="authForm"
    @submit-auth="submitAuth"
    @oauth-login="handleOAuthLogin"
  />

  <main v-else class="app-shell">
    <header class="topbar">
      <div class="brand compact-brand">
        <div class="brand-mark">SY</div>
        <div>
          <strong>SYCHAT</strong>
          <span>{{ currentUser.username || "用户" }}</span>
        </div>
      </div>

      <nav class="mode-tabs" style="padding: 10px;">
        <button :class="{ active: viewMode === 'chat' }" @click="viewMode = 'chat'">
          <MessageCircle :size="13" />
          聊天
        </button>
        <button :class="{ active: viewMode === 'watch' }" @click="viewMode = 'watch'">
          <Film :size="13" />
          一起看
        </button>
      </nav>

      <div class="top-actions">
        <button class="status-line status-action" :title="wsStatusText" @click="retryWsConnection">
          <span :class="['dot', { ok: wsConnected, warn: wsReconnecting, fail: wsReconnectFailed }]"></span>
          <span>{{ wsStatusText }}</span>
        </button>
        <button class="ghost" @click="logout">退出</button>
      </div>
    </header>

    <section v-if="viewMode === 'chat'" class="chat-workspace">
      <aside class="chat-directory">
        <div class="directory-tabs">
          <button :class="{ active: directoryMode === 'conversations' }" @click="directoryMode = 'conversations'">
            会话
          </button>
          <button :class="{ active: directoryMode === 'friends' }" @click="directoryMode = 'friends'">好友</button>
        </div>

        <ConversationList
          v-if="directoryMode === 'conversations'"
          :token="token"
          :conversations="conversations"
          :active-conversation="activeConversation"
          @load-offline="loadOffline"
          @select-conversation="selectConversation"
        />

        <FriendsPanel
          v-else
          :token="token"
          :friends="friends"
          :friend-requests="friendRequests"
          :friend-form="friendForm"
          :format-time="formatTime"
          @refresh="refreshFriends"
          @submit-request="submitFriendRequest"
          @operate-request="handleFriendRequest"
          @open-chat="openPrivateConversation"
        />
      </aside>

      <ChatPanel
        v-model:message-text="messageText"
        :active-conversation="activeConversation"
        :messages="messages"
        :current-user="currentUser"
        :ws-connected="wsConnected"
        :message-list-ref="messageList"
        @send-message="sendMessage"
      />
    </section>

    <section
      v-else
      :class="['watch-workspace', { 'chat-collapsed': watchChatCollapsed }]"
      :style="{ gridTemplateColumns: watchChatCollapsed ? '44px minmax(0, 1fr)' : `${watchSidebarWidth}px minmax(0, 1fr)` }"
    >
      <aside class="watch-chat-sidebar">
        <button class="collapse-tab" @click="watchChatCollapsed = !watchChatCollapsed">
          <PanelLeftClose v-if="!watchChatCollapsed" :size="18" />
          <PanelLeftOpen v-else :size="18" />
        </button>

        <div v-if="!watchChatCollapsed" class="watch-resize-handle" @pointerdown="startWatchSidebarResize"></div>

        <template v-if="!watchChatCollapsed">
          <ConversationList
            :token="token"
            :conversations="conversations"
            :active-conversation="activeConversation"
            @load-offline="loadOffline"
            @select-conversation="selectConversation"
          />

          <ChatPanel
            v-model:message-text="messageText"
            :active-conversation="activeConversation"
            :messages="messages"
            :current-user="currentUser"
            :ws-connected="wsConnected"
            :message-list-ref="messageList"
            @send-message="sendMessage"
          />
        </template>
      </aside>

      <WatchPanel
        v-model:file-id-input="fileIdInput"
        :token="token"
        :active-room-id="activeRoomId"
        :room-form="roomForm"
        :upload-name="uploadName"
        :video="video"
        :visible-danmaku="visibleDanmaku"
        @update:video-el="setVideoElement"
        @watch-control="sendWatchControl"
        @seek-by="seekBy"
        @video-time-update="onVideoTimeUpdate"
        @create-room="handleCreateRoom"
        @join-room="handleJoinRoom"
        @invite="handleInvite"
        @upload="handleUpload"
        @load-file="loadFile"
        @load-video="loadVideoToRoom"
      />
    </section>
  </main>

  <div v-if="toast" class="toast">{{ toast }}</div>
</template>

<script setup>
import { onBeforeUnmount, ref } from "vue";
import ChatPanel from "./components/ChatPanel.vue";
import ConversationList from "./components/ConversationList.vue";
import FriendsPanel from "./components/FriendsPanel.vue";
import LoginPage from "./components/LoginPage.vue";
import WatchPanel from "./components/WatchPanel.vue";
import { useImClient } from "./composables/useImClient";
import { Film, MessageCircle, PanelLeftClose, PanelLeftOpen } from "@lucide/vue";

const viewMode = ref("chat");
const directoryMode = ref("conversations");
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
  video,
  visibleDanmaku,
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
  seekBy,
  onVideoTimeUpdate,
  setVideoElement,
  formatTime,
} = useImClient();


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
