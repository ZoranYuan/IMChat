<template>
  <section class="chat-pane">
    <header class="chat-head">
      <div>
        <p class="eyebrow">Chat Room</p>
        <h2>{{ activeConversation?.conversationId || "选择或打开一个会话" }}</h2>
      </div>
      <span class="pill">{{ activeConversation?.convType === 2 ? "群聊" : "私聊" }}</span>
    </header>

    <div class="messages" :ref="setMessageList">
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
        :value="messageText"
        :disabled="!activeConversation || !wsConnected"
        placeholder="输入消息"
        @input="$emit('update:messageText', $event.target.value)"
        @keydown.enter="$emit('send-message')"
      />
      <button class="primary" :disabled="!activeConversation || !wsConnected" @click="$emit('send-message')">
        <Send :size="16" />
        发送
      </button>
    </footer>
  </section>
</template>

<script setup>
import { MessageCircle, Send } from "@lucide/vue";

const props = defineProps({
  activeConversation: { type: Object, default: null },
  messages: { type: Array, default: () => [] },
  currentUser: { type: Object, required: true },
  messageText: { type: String, default: "" },
  wsConnected: { type: Boolean, default: false },
  formatTime: { type: Function, required: true },
  messageListRef: { type: Object, required: true },
});

defineEmits(["update:messageText", "send-message"]);

function setMessageList(el) {
  props.messageListRef.value = el;
}
</script>
