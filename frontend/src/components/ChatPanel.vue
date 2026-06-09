<template>
  <section v-if="!activeConversation" key="placeholder" class="chat-pane chat-placeholder">
    <div class="empty-chat">
      <MessageCircle :size="54" />
      <p>SYCHAT</p>
    </div>
  </section>

  <section v-else key="conversation" class="chat-pane">
    <header class="chat-head">
      <div class="chat-head-main">
        <h2>{{ activeConversation.displayName }}</h2>
      </div>
    </header>

    <div class="messages" :ref="setMessageList">
      <div v-if="conversationLoading" class="messages-loading">
        <span class="loading-dot"></span>
        <span>正在加载聊天记录</span>
      </div>
      <template v-for="(msg, index) in messages" :key="msg.clientMsgId || msg.messageId || msg.seq || index">
        <article :class="['bubble-row', { mine: isMine(msg), group: isGroupChat }]">
          <div class="bubble-avatar" :class="{ mine: isMine(msg) }">
            <img v-if="avatarUrl(msg)" :src="avatarUrl(msg)" :alt="senderName(msg)" />
            <span v-else>{{ avatarText(msg) }}</span>
          </div>

          <div class="bubble-stack" :class="{ mine: isMine(msg) }">
            <div v-if="isGroupChat && !isMine(msg)" class="bubble-sender">{{ senderName(msg) }}</div>
            <div class="bubble-shell" :class="{ mine: isMine(msg) }">
              <div class="bubble-content">{{ msg.content }}</div>
            </div>
          </div>
        </article>

        <div v-if="isReadBoundary(index)" class="read-boundary" :class="{ group: isGroupChat }">
          <span class="read-boundary-line"></span>
          <span class="read-boundary-pill">
            <span v-if="readReceiversPreview.length" class="read-avatars">
              <span v-for="reader in readReceiversPreview" :key="reader.userId" class="read-avatar">
                <img v-if="reader.avatar" :src="reader.avatar" :alt="reader.userId" />
                <span v-else>{{ readAvatarText(reader) }}</span>
              </span>
            </span>
            <span class="read-boundary-text">已读</span>
          </span>
          <span class="read-boundary-line"></span>
        </div>
      </template>
    </div>

    <footer class="composer">
      <div class="composer-shell">
        <textarea ref="messageInputRef" :style="{ height: `${composerHeight}px` }" :value="messageText"
          :disabled="!wsConnected" rows="1" @input="handleMessageInput"
          @keydown.enter.exact.prevent="handleEnterSend" @keydown.enter.shift.stop></textarea>
        <div class="composer-top-resizer" @pointerdown="startComposerResize"></div>

        <div class="composer-toolbar">
          <div class="composer-tools">
            <button class="composer-tool" type="button" :disabled="!wsConnected" title="表情">
              <Smile :size="16" />
            </button>
            <button class="composer-tool" type="button" :disabled="!wsConnected" title="文件">
              <Paperclip :size="16" />
            </button>
            <button class="composer-tool" type="button" :disabled="!wsConnected" title="更多">
              <MoreHorizontal :size="16" />
            </button>
          </div>

          <div class="composer-actions">
            <button class="composer-tool composer-audio" type="button" :disabled="!wsConnected" title="语音">
              <Volume2 :size="16" />
            </button>
            <button v-if="showWatchEntry && activeConversation.convType === 2" class="composer-watch-entry"
              type="button" :title="watchEntryHint" :disabled="watchEntryDisabled" @click="$emit('open-watch')">
              <Film :size="14" />
              {{ watchEntryLabel }}
            </button>
            <button class="composer-send" type="button" :disabled="!wsConnected" @click="$emit('send-message')">
              <Send :size="16" />
              发送
            </button>
          </div>
        </div>
      </div>
    </footer>
  </section>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Film, MessageCircle, MoreHorizontal, Paperclip, Send, Smile, Volume2 } from "@lucide/vue";

const props = defineProps({
  activeConversation: { type: Object, default: null },
  messages: { type: Array, default: () => [] },
  conversationLoading: { type: Boolean, default: false },
  currentUser: { type: Object, required: true },
  messageText: { type: String, default: "" },
  wsConnected: { type: Boolean, default: false },
  setMessageListRef: { type: Function, required: true },
  lastReadSeq: { type: Number, default: 0 },
  readReceivers: { type: Array, default: () => [] },
  showWatchEntry: { type: Boolean, default: false },
  watchEntryLabel: { type: String, default: "一起看" },
  watchEntryHint: { type: String, default: "" },
  watchEntryDisabled: { type: Boolean, default: false },
});

const emit = defineEmits(["update:messageText", "send-message", "open-watch"]);

const isGroupChat = computed(() => props.activeConversation?.convType === 2);
const readReceiversPreview = computed(() => (Array.isArray(props.readReceivers) ? props.readReceivers.slice(0, 4) : []));
const readBoundaryIndex = computed(() => {
  const maxSeq = Number(props.lastReadSeq) || 0;
  if (!maxSeq) return -1;
  let boundary = -1;
  props.messages.forEach((msg, index) => {
    if (!isMine(msg)) return;
    const seq = Number(msg.seq) || 0;
    if (seq > 0 && seq <= maxSeq) boundary = index;
  });
  return boundary;
});
const messageInputRef = ref(null);
const composerMinHeight = 124;
const composerMaxHeight = 180;
const composerHeight = ref(composerMinHeight);
let composerResizeStartY = 0;
let composerResizeStartHeight = composerMinHeight;

function setMessageList(el) {
  props.setMessageListRef(el);
}

function isMine(msg) {
  return msg.senderId === props.currentUser.userId;
}

function senderName(msg) {
  return msg.senderUsername || msg.username || (isMine(msg) ? props.currentUser.username : "成员");
}

function avatarText(msg) {
  const name = senderName(msg);
  if (!name) return "?";
  return name.slice(0, 2).toUpperCase();
}

function avatarUrl(msg) {
  return msg.senderAvatar || msg.avatar || (isMine(msg) ? props.currentUser.avatar : "");
}

function readAvatarText(reader) {
  const text = String(reader?.userId || "");
  if (!text) return "已";
  return text.slice(0, 2).toUpperCase();
}

function isReadBoundary(index) {
  return index === readBoundaryIndex.value;
}

function resizeMessageInput() {
  const el = messageInputRef.value;
  if (!el) return;
  const nextHeight = Math.min(
    composerMaxHeight,
    Math.max(composerMinHeight, Math.max(el.scrollHeight, composerHeight.value)),
  );
  composerHeight.value = nextHeight;
  el.style.overflowY = el.scrollHeight > composerMaxHeight ? "auto" : "hidden";
}

function handleMessageInput(event) {
  resizeMessageInput();
  const target = event.target;
  emit("update:messageText", target.value);
}

function handleEnterSend(event) {
  if (event.isComposing) return;
  emit("send-message");
}

function startComposerResize(event) {
  if (!props.wsConnected) return;
  event.preventDefault();
  composerResizeStartY = event.clientY;
  composerResizeStartHeight = composerHeight.value;
  window.addEventListener("pointermove", resizeComposerHeight);
  window.addEventListener("pointerup", stopComposerResize, { once: true });
}

function resizeComposerHeight(event) {
  const delta = composerResizeStartY - event.clientY;
  const nextHeight = Math.min(composerMaxHeight, Math.max(composerMinHeight, composerResizeStartHeight + delta));
  composerHeight.value = nextHeight;
  nextTick(() => resizeMessageInput());
}

function stopComposerResize() {
  window.removeEventListener("pointermove", resizeComposerHeight);
}

watch(
  () => props.messageText,
  () => {
    nextTick(() => resizeMessageInput());
  },
);

watch(
  () => props.activeConversation?.conversationId,
  () => {
    composerHeight.value = composerMinHeight;
    nextTick(() => resizeMessageInput());
  },
);

onMounted(() => {
  resizeMessageInput();
});

onBeforeUnmount(() => {
  window.removeEventListener("pointermove", resizeComposerHeight);
});
</script>

<style scoped>
.chat-pane {
  display: grid;
  grid-template-rows: 76px minmax(0, 1fr) auto;
  min-width: 0;
  min-height: 0;
  max-height: calc(100dvh - 24px);
  background: rgba(255, 255, 255, 0.02);
}

.chat-placeholder {
  grid-template-rows: 1fr;
}

.empty-chat {
  display: grid;
  place-items: center;
  align-content: center;
  gap: 14px;
  height: 100%;
  color: var(--muted);
}

.empty-chat p {
  color: var(--muted);
  font-size: 13px;
  letter-spacing: 0.12em;
}

.chat-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 18px 24px;
  border-bottom: 1px solid var(--border);
  background: var(--surface-soft);
}

.chat-head-main {
  min-width: 0;
}

.chat-head h2 {
  margin: 0;
  font-size: 20px;
  letter-spacing: -0.02em;
}

.chat-head-subtitle {
  display: inline-flex;
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
}

.messages {
  min-height: 0;
  max-height: min(100%, calc(100dvh - 180px));
  overflow: auto;
  padding: 16px 16px 20px;
}

.messages-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 120px;
  color: var(--muted);
  font-size: 13px;
}

.loading-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--primary);
  box-shadow: 0 0 0 6px color-mix(in srgb, var(--primary) 18%, transparent);
  animation: loadingPulse 1.1s ease-in-out infinite;
}

.bubble-row {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin-bottom: 25px;
}

.bubble-row.mine {
  flex-direction: row-reverse;
}

.bubble-avatar {
  flex: none;
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 30%;
  overflow: hidden;
  background: var(--chat-avatar-other-bg);
  color: #fff;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.04em;
  box-shadow: var(--shadow-md);
}

.bubble-avatar.mine {
  background: linear-gradient(135deg, var(--primary), var(--primary-strong));
}

.bubble-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.bubble-stack {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-width: 0;
  max-width: min(70%, 680px);
}

.bubble-stack.mine {
  align-items: flex-end;
}

.bubble-sender {
  margin: 0 0 3px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1;
}

.bubble-shell {
  position: relative;
  min-width: 0;
  overflow: visible;
}

.bubble-content {
  position: relative;
  z-index: 1;
  padding: 9px 12px;
  border: 1px solid var(--chat-bubble-other-border);
  border-radius: 10px;
  background: var(--chat-bubble-other-bg);
  color: var(--text);
  font-size: 13.5px;
  box-shadow: var(--shadow-md);
  word-break: break-word;
  white-space: pre-wrap;
}

.bubble-shell::before {
  content: "";
  position: absolute;
  top: 13px;
  left: -5px;
  width: 0;
  height: 0;
  border-top: 6px solid transparent;
  border-bottom: 6px solid transparent;
  border-right: 6px solid var(--chat-bubble-other-tail);
  filter: drop-shadow(-1px 0 0 var(--chat-bubble-other-border));
  z-index: 0;
}

.bubble-shell.mine::before {
  left: auto;
  right: -5px;
  border-right: 0;
  border-left: 6px solid var(--chat-bubble-mine-tail);
}

.bubble-shell.mine .bubble-content {
  border-color: var(--chat-bubble-mine-border);
  background: var(--chat-bubble-mine-bg);
  color: #fff;
  text-shadow: 0 1px 1px rgba(0, 0, 0, 0.12);
}

.read-boundary {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: -8px 0 20px;
  padding-left: 44px;
}

.read-boundary.group {
  padding-left: 44px;
}

.read-boundary-line {
  flex: 1;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--border), transparent);
}

.read-boundary-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(141, 91, 255, 0.08);
  border: 1px solid color-mix(in srgb, var(--primary) 20%, transparent);
  color: var(--muted);
  font-size: 12px;
  line-height: 1;
  white-space: nowrap;
}

.read-avatars {
  display: inline-flex;
  align-items: center;
  margin-right: 2px;
}

.read-avatar {
  width: 18px;
  height: 18px;
  margin-left: -5px;
  border-radius: 999px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.85);
  background: linear-gradient(135deg, var(--primary), var(--primary-strong));
  color: #fff;
  font-size: 9px;
  font-weight: 700;
  display: grid;
  place-items: center;
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.12);
}

.read-avatar:first-child {
  margin-left: 0;
}

.read-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.composer {
  padding: 12px 14px 14px;
  border-top: 1px solid var(--border);
  background: var(--surface-soft);
}

.composer-shell {
  position: relative;
  display: grid;
  grid-template-rows: minmax(124px, 1fr) auto;
  gap: 8px;
  padding: 14px 12px 12px;
  border: 1px solid var(--border);
  border-radius: 20px;
  background: var(--composer-shell-bg);
  box-shadow: var(--shadow-md);
}

.composer-shell textarea {
  width: 100%;
  min-height: 124px;
  max-height: 180px;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--text);
  resize: none;
  outline: none;
  font-size: 14px;
  line-height: 1.65;
  overflow-y: hidden;
}

.composer-shell textarea::placeholder {
  color: var(--muted);
}

.composer-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 34px;
  padding-top: 6px;
  border-top: 1px solid var(--composer-shell-border);
}

.composer-tools,
.composer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.composer-tool {
  width: 32px;
  height: 32px;
  min-height: 32px;
  border-radius: 10px;
  border: 0;
  background: transparent;
  color: var(--muted);
}

.composer-tool:hover {
  background: rgba(141, 91, 255, 0.08);
  color: var(--text);
}

.composer-watch-entry {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: rgba(141, 91, 255, 0.08);
  color: var(--text);
}

.composer-send {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 15px;
  border: 0;
  border-radius: 10px;
  background: var(--primary);
  color: #fff;
  box-shadow: var(--shadow-md);
}

.composer-send:hover {
  transform: translateY(-1px);
}

.composer-send:disabled,
.composer-tool:disabled,
.composer-watch-entry:disabled {
  opacity: 0.46;
}

.composer-audio {
  width: 30px;
  background: transparent;
}

.composer-top-resizer {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 10px;
  cursor: ns-resize;
  background: transparent;
  z-index: 2;
}

@media (max-width: 1180px) {
  .chat-pane {
    min-height: 620px;
  }
}

@keyframes loadingPulse {
  0%,
  100% {
    transform: scale(0.88);
    opacity: 0.7;
  }
  50% {
    transform: scale(1);
    opacity: 1;
  }
}
</style>
