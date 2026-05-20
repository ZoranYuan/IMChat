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
          <span>{{ currentUser.userId }}</span>
        </div>
      </div>

      <nav class="mode-tabs">
        <button :class="{ active: viewMode === 'chat' }" @click="viewMode = 'chat'">
          <MessageCircle :size="17" />
          聊天
        </button>
        <button :class="{ active: viewMode === 'watch' }" @click="viewMode = 'watch'">
          <Film :size="17" />
          一起看
        </button>
      </nav>

      <div class="top-actions">
        <div class="status-line">
          <span :class="['dot', wsConnected ? 'ok' : '']"></span>
          <span>{{ wsConnected ? "实时在线" : "未连接" }}</span>
        </div>
        <button class="ghost" @click="connectWs">
          <RadioTower :size="16" />
          连接
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
          v-model:manual-conversation-id="manualConversationId"
          v-model:manual-conv-type="manualConvType"
          :token="token"
          :conversations="conversations"
          :active-conversation="activeConversation"
          @load-offline="loadOffline"
          @open-manual="openManualConversation"
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
        :format-time="formatTime"
        :message-list-ref="messageList"
        @send-message="sendMessage"
      />
    </section>

    <section v-else :class="['watch-workspace', { 'chat-collapsed': watchChatCollapsed }]">
      <aside class="watch-chat-sidebar">
        <button class="collapse-tab" @click="watchChatCollapsed = !watchChatCollapsed">
          <PanelLeftClose v-if="!watchChatCollapsed" :size="18" />
          <PanelLeftOpen v-else :size="18" />
        </button>

        <template v-if="!watchChatCollapsed">
          <ConversationList
            v-model:manual-conversation-id="manualConversationId"
            v-model:manual-conv-type="manualConvType"
            :token="token"
            :conversations="conversations"
            :active-conversation="activeConversation"
            @load-offline="loadOffline"
            @open-manual="openManualConversation"
            @select-conversation="selectConversation"
          />

          <ChatPanel
            v-model:message-text="messageText"
            :active-conversation="activeConversation"
            :messages="messages"
            :current-user="currentUser"
            :ws-connected="wsConnected"
            :format-time="formatTime"
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
        :video-ref="videoRef"
        :visible-danmaku="visibleDanmaku"
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

    <div v-if="toast" class="toast">{{ toast }}</div>
  </main>
</template>

<script setup>
import { ref } from "vue";
import ChatPanel from "./components/ChatPanel.vue";
import ConversationList from "./components/ConversationList.vue";
import FriendsPanel from "./components/FriendsPanel.vue";
import LoginPage from "./components/LoginPage.vue";
import WatchPanel from "./components/WatchPanel.vue";
import { useImClient } from "./composables/useImClient";
import { Film, MessageCircle, PanelLeftClose, PanelLeftOpen, RadioTower } from "@lucide/vue";

const viewMode = ref("chat");
const directoryMode = ref("conversations");
const watchChatCollapsed = ref(false);

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
  manualConversationId,
  manualConvType,
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
  openManualConversation,
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
} = useImClient();

function handleOAuthLogin(provider) {
  const providerName = provider === "wechat" ? "微信" : "GitHub";
  toast.value = `${providerName} 登录后端接口待接入`;
}

function refreshFriends() {
  loadFriends();
  loadFriendRequests();
}
</script>
