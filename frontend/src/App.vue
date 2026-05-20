<template>
  <main class="shell">
    <SidebarAuth
      v-model:auth-mode="authMode"
      :token="token"
      :current-user="currentUser"
      :auth-form="authForm"
      :ws-connected="wsConnected"
      @submit-auth="submitAuth"
      @connect-ws="connectWs"
    />

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

    <div v-if="toast" class="toast">{{ toast }}</div>
  </main>
</template>

<script setup>
import ChatPanel from "./components/ChatPanel.vue";
import ConversationList from "./components/ConversationList.vue";
import SidebarAuth from "./components/SidebarAuth.vue";
import WatchPanel from "./components/WatchPanel.vue";
import { useImClient } from "./composables/useImClient";

const {
  token,
  currentUser,
  authMode,
  authForm,
  conversations,
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
  selectConversation,
  openManualConversation,
  connectWs,
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
</script>
