<template>
  <section v-if="!activeConversation" class="chat-pane chat-placeholder">
    <div class="empty-chat">
      <MessageCircle :size="54" />
      <p>SYCHAT</p>
    </div>
  </section>

  <section v-else class="chat-pane">
    <header class="chat-head">
      <div>
        <h2>{{ activeConversation.displayName || "会话" }}</h2>
      </div>
      <div class="chat-head-actions">
        <span class="pill">{{ activeConversation.convType === 2 ? "群聊" : "私聊" }}</span>
        <button
          v-if="showWatchEntry && activeConversation.convType === 2"
          class="ghost watch-entry-btn"
          :title="watchEntryHint"
          :disabled="watchEntryDisabled"
          @click="$emit('open-watch')"
        >
          <Film :size="16" />
          {{ watchEntryLabel }}
        </button>
      </div>
    </header>

    <div class="messages" :ref="setMessageList">
      <article
        v-for="msg in messages"
        :key="msg.clientMsgId || msg.messageId"
        :class="['bubble-row', msg.senderId === currentUser.userId ? 'mine' : '']"
      >
        <div class="bubble">
          <div v-if="showSenderName(msg)" class="bubble-meta">
            <span>{{ msg.senderUsername || msg.username || "成员" }}</span>
          </div>
          <p>{{ msg.content }}</p>
        </div>
      </article>
    </div>

    <footer class="composer">
      <input
        :value="messageText"
        :disabled="!wsConnected"
        placeholder="输入消息"
        @input="$emit('update:messageText', $event.target.value)"
        @keydown.enter="$emit('send-message')"
      />
      <button class="primary" :disabled="!wsConnected" @click="$emit('send-message')">
        <Send :size="16" />
        发送
      </button>
    </footer>
  </section>
</template>

<script setup>
import { Film, MessageCircle, Send } from "@lucide/vue";

const props = defineProps({
  activeConversation: { type: Object, default: null },
  messages: { type: Array, default: () => [] },
  currentUser: { type: Object, required: true },
  messageText: { type: String, default: "" },
  wsConnected: { type: Boolean, default: false },
  messageListRef: { type: Object, required: true },
  showWatchEntry: { type: Boolean, default: false },
  watchEntryLabel: { type: String, default: "一起看" },
  watchEntryHint: { type: String, default: "" },
  watchEntryDisabled: { type: Boolean, default: false },
});

defineEmits(["update:messageText", "send-message", "open-watch"]);

function setMessageList(el) {
  props.messageListRef.value = el;
}

function showSenderName(msg) {
  return props.activeConversation?.convType === 2 && msg.senderId !== props.currentUser.userId;
}
</script>

<style scoped>
.chat-pane {
  display: grid;
  grid-template-rows: 76px minmax(0, 1fr) 78px;
  min-width: 0;
  min-height: 0;
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

.chat-head h2 {
  margin: 0;
  font-size: 20px;
  letter-spacing: -0.02em;
}

.chat-head-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.watch-entry-btn {
  min-height: 38px;
  padding: 0 14px;
  border-radius: 999px;
  background: var(--primary-soft);
  color: var(--text);
}

.messages {
  min-height: 0;
  overflow: auto;
  padding: 24px;
}

.bubble-row {
  display: flex;
  margin-bottom: 14px;
}

.bubble-row.mine {
  justify-content: flex-end;
}

.bubble {
  max-width: min(72%, 620px);
  border: 1px solid var(--border);
  border-radius: 22px 22px 22px 8px;
  padding: 12px 14px;
  background: var(--surface-strong);
  color: var(--text);
  line-height: 1.6;
  box-shadow: var(--shadow-md);
}

.bubble-row.mine .bubble {
  border-color: transparent;
  background: linear-gradient(135deg, var(--primary), var(--primary-strong));
  color: #fff;
  border-radius: 22px 22px 8px 22px;
}

.bubble-meta {
  display: flex;
  gap: 10px;
  margin-bottom: 5px;
  color: rgba(255, 255, 255, 0.72);
  font-size: 11px;
}

.bubble-row:not(.mine) .bubble-meta {
  color: var(--muted);
}

.composer {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 104px;
  gap: 10px;
  padding: 16px 24px;
  border-top: 1px solid var(--border);
  background: var(--surface-soft);
}

.composer input {
  height: 44px;
}

@media (max-width: 1180px) {
  .chat-pane {
    min-height: 620px;
  }

  .composer {
    grid-template-columns: minmax(0, 1fr) 90px;
  }
}
</style>
