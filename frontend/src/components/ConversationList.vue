<template>
  <section class="conversation-pane">
    <div class="pane-head">
      <div>
        <h1>消息</h1>
      </div>
      <button class="icon-btn" :disabled="!token" title="刷新会话" @click="$emit('load-offline')">
        <RefreshCcw :size="18" />
      </button>
    </div>

    <div class="conversation-search">
      <Search :size="16" />
      <input placeholder="搜索" />
    </div>

    <div class="conversation-list">
      <div v-if="!conversations.length" class="list-empty">暂无会话</div>
      <button
        v-for="item in conversations"
        :key="item.conversationId"
        :class="['conversation-item', activeConversation?.conversationId === item.conversationId ? 'active' : '']"
        @click="$emit('select-conversation', item)"
      >
        <div class="avatar">{{ displayName(item).slice(0, 2).toUpperCase() }}</div>
        <div class="item-main">
          <div class="item-row">
            <strong>{{ displayName(item) }}</strong>
            <span v-if="item.unread" class="badge">{{ item.unread }}</span>
          </div>
          <p>{{ item.latestMessage?.content || "暂无消息" }}</p>
        </div>
      </button>
    </div>
  </section>
</template>

<script setup>
import { RefreshCcw, Search } from "@lucide/vue";

defineProps({
  token: { type: String, default: "" },
  conversations: { type: Array, default: () => [] },
  activeConversation: { type: Object, default: null },
});

defineEmits(["load-offline", "select-conversation"]);

function displayName(item) {
  return item.displayName || item.latestMessage?.displayName || "会话";
}
</script>
