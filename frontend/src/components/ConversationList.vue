<template>
  <section class="conversation-pane">
    <div class="pane-head">
      <div>
        <p class="eyebrow">Conversations</p>
        <h1>消息</h1>
      </div>
      <button class="icon-btn" :disabled="!token" title="刷新会话" @click="$emit('load-offline')">
        <RefreshCcw :size="18" />
      </button>
    </div>

    <div class="quick-form">
      <input
        :value="manualConversationId"
        placeholder="会话 / 房间 ID"
        @input="$emit('update:manualConversationId', $event.target.value.trim())"
      />
      <select :value="manualConvType" @change="$emit('update:manualConvType', Number($event.target.value))">
        <option :value="1">私聊</option>
        <option :value="2">群聊</option>
      </select>
      <button class="ghost" @click="$emit('open-manual')">打开</button>
    </div>

    <div class="conversation-list">
      <button
        v-for="item in conversations"
        :key="item.conversationId"
        :class="['conversation-item', activeConversation?.conversationId === item.conversationId ? 'active' : '']"
        @click="$emit('select-conversation', item)"
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
</template>

<script setup>
import { RefreshCcw } from "@lucide/vue";

defineProps({
  token: { type: String, default: "" },
  conversations: { type: Array, default: () => [] },
  activeConversation: { type: Object, default: null },
  manualConversationId: { type: String, default: "" },
  manualConvType: { type: Number, default: 2 },
});

defineEmits([
  "load-offline",
  "open-manual",
  "select-conversation",
  "update:manualConversationId",
  "update:manualConvType",
]);
</script>
