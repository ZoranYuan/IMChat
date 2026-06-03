<template>
  <section class="conversation-pane">
    <div class="pane-head">
      <div>
        <h1>消息列表</h1>
      </div>
      <button class="icon-btn" :disabled="!token || refreshing" :aria-busy="refreshing" title="刷新会话"
        @click="$emit('load-offline')">
        <RefreshCcw :class="{ spinning: refreshing }" :size="13" />
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
        <div class="avatar conversation-avatar">
          <img v-if="showAvatar(item)" :src="item.avatar" :alt="displayName(item)" @error="handleAvatarError(item)" />
          <span v-else>{{ avatarText(item) }}</span>
        </div>
        <div class="item-main">
          <div class="item-row">
            <strong>{{ displayName(item) }}</strong>
          </div>
          <span v-if="item.unread" class="badge unread-badge">{{ item.unread }}</span>
          <p>{{ item.latestMessage?.content || "暂无消息" }}</p>
        </div>
      </button>
    </div>
  </section>
</template>

<script setup>
import { ref } from "vue";
import { RefreshCcw, Search } from "@lucide/vue";

defineProps({
  token: { type: String, default: "" },
  conversations: { type: Array, default: () => [] },
  activeConversation: { type: Object, default: null },
  refreshing: { type: Boolean, default: false },
});

defineEmits(["load-offline", "select-conversation"]);

const brokenAvatarSources = ref(new Set());

function displayName(item) {
  return item.displayName || item.latestMessage?.displayName || "会话";
}

function showAvatar(item) {
  return Boolean(item.avatar) && !brokenAvatarSources.value.has(item.avatar);
}

function handleAvatarError(item) {
  if (!item?.avatar) return;
  if (brokenAvatarSources.value.has(item.avatar)) return;
  const next = new Set(brokenAvatarSources.value);
  next.add(item.avatar);
  brokenAvatarSources.value = next;
}

function avatarText(item) {
  return displayName(item).slice(0, 2).toUpperCase();
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
  position: relative;
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

.conversation-avatar {
  overflow: hidden;
  background: var(--primary-soft);
}

.conversation-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.conversation-item:hover,
.conversation-item.active {
  border-color: var(--border-strong);
  background: var(--surface-strong);
  box-shadow: var(--shadow-md);
}

.item-main {
  position: relative;
  min-width: 0;
  flex: 1;
  padding-right: 44px;
}

.item-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.unread-badge {
  position: absolute;
  top: 2px;
  right: 0;
  min-width: 22px;
  padding: 2px 7px;
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

.spinning {
  animation: spin 0.8s linear infinite;
  transform-origin: center;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
