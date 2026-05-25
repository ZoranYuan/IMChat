<template>
  <section class="conversation-pane">
    <div class="pane-head">
      <div>
        <h1>消息</h1>
      </div>
      <button class="icon-btn" :disabled="!token" title="刷新会话" @click="$emit('load-offline')">
        <RefreshCcw :size="13" />
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

<style scoped>
.conversation-pane {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  padding: 22px 18px;
  background: var(--surface-soft);
}

.pane-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.pane-head h1 {
  margin: 0;
  font-size: 22px;
  letter-spacing: -0.02em;
}

.conversation-search {
  position: relative;
  margin: 16px 0 14px;
}

.conversation-search svg {
  position: absolute;
  top: 50%;
  left: 12px;
  color: var(--muted);
  transform: translateY(-50%);
}

.conversation-search input {
  height: 40px;
  padding-left: 38px;
}

.conversation-list {
  min-height: 0;
  overflow: auto;
  padding-right: 4px;
}

.list-empty {
  padding: 28px 0;
  color: var(--muted);
  font-size: 13px;
  text-align: center;
}

.conversation-item {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 74px;
  margin-bottom: 10px;
  padding: 12px;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  background: var(--surface);
  color: var(--text);
  text-align: left;
}

.conversation-item:hover,
.conversation-item.active {
  border-color: var(--border-strong);
  background: var(--surface-strong);
  box-shadow: var(--shadow-md);
}

.item-main {
  min-width: 0;
  flex: 1;
}

.item-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.item-row strong,
.item-main p {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-main p {
  margin-top: 4px;
  color: var(--muted);
  font-size: 13px;
}
</style>
