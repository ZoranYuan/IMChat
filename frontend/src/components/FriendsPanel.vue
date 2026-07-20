<template>
  <section class="friends-pane">
    <div class="pane-head">
      <div>
        <p class="eyebrow">Contacts</p>
        <h1>好友</h1>
      </div>
      <button class="icon-btn" :disabled="!token || refreshing" :aria-busy="refreshing" title="刷新好友"
        @click="$emit('refresh')">
        <RefreshCcw :class="{ spinning: refreshing }" :size="13" />
      </button>
    </div>

    <div class="friend-form">
      <input :value="friendForm.keyword" placeholder="用户名 / 手机号"
        @input="friendForm.keyword = $event.target.value.trim()" :style="{ fontSize: '14px' }" />
      <input :value="friendForm.message" placeholder="申请留言" @input="friendForm.message = $event.target.value"
        :style="{ fontSize: '14px' }" />
      <button class="primary wide" :disabled="!token" @click="$emit('submit-request')">
        <UserPlus :size="15" />
        发送好友申请
      </button>
    </div>

    <div class="friend-section">
      <div class="section-title">
        <span>好友列表</span>
        <span>{{ friendConvs.length }}</span>
      </div>
      <button v-for="friendConv in friendConvs" :key="friendConv.friendUserId" class="friend-item"
        @click="$emit('open-chat', friendConv)">
        <div class="avatar">{{ displayName(friendConv).slice(0, 2).toUpperCase() }}</div>
        <div class="item-main">
          <strong>{{ displayName(friendConv) }}</strong>
          <p>{{ friendConv.friendUsername || displayName(friendConv) }}</p>
        </div>
        <MessageCircle :size="17" />
      </button>
      <p v-if="friendConvs.length === 0" class="empty-hint">暂无好友</p>
    </div>

    <div class="friend-section">
      <div class="section-title">
        <span>好友申请</span>
        <span>{{ pendingRequests.length }}</span>
      </div>
      <article v-for="request in pendingRequests" :key="request.requestId" class="request-item">
        <div>
          <strong>{{ request.fromDisplayName || request.fromUsername || "用户" }}</strong>
          <p>{{ formatTime(request.applyTime) }}</p>
        </div>
        <div class="request-actions">
          <button class="ghost" @click="$emit('operate-request', request.requestId, request.fromUserId, 1)">同意</button>
          <button class="ghost" @click="$emit('operate-request', request.requestId, request.fromUserId, 2)">拒绝</button>
        </div>
      </article>
      <p v-if="pendingRequests.length === 0" class="empty-hint">暂无待处理申请</p>
    </div>
  </section>
</template>

<script setup>
import { MessageCircle, RefreshCcw, UserPlus } from "@lucide/vue";
import { computed } from "vue";

const props = defineProps({
  token: { type: String, default: "" },
  friendConvs: { type: Array, default: () => [] },
  friendRequests: { type: Array, default: () => [] },
  friendForm: { type: Object, required: true },
  formatTime: { type: Function, required: true },
  refreshing: { type: Boolean, default: false },
});

defineEmits(["refresh", "submit-request", "operate-request", "open-chat"]);

// 过滤出未处理的好友申请
const pendingRequests = computed(() => props.friendRequests.filter((request) => request.status === 1));

function displayName(friend) {
  return friend.displayName || friend.friendUsername || "好友";
}
</script>

<style scoped>
.friends-pane {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  padding: 22px 18px;
  overflow: auto;
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

.eyebrow {
  margin: 0 0 4px;
  color: var(--primary);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.friend-form {
  display: grid;
  gap: 10px;
  margin: 18px 0 16px;
}

.friend-section {
  display: grid;
  gap: 8px;
  margin-bottom: 18px;
}

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--muted);
  font-size: 12px;
}

.friend-item,
.request-item {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 66px;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  padding: 12px;
  background: var(--surface);
  color: var(--text);
  text-align: left;
}

.friend-item:hover {
  border-color: var(--border-strong);
  background: var(--surface-strong);
  box-shadow: var(--shadow-md);
}

.request-item {
  justify-content: space-between;
}

.request-item p,
.friend-item p,
.empty-hint {
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
}

.request-actions {
  display: flex;
  gap: 6px;
}

.request-actions .ghost {
  height: 32px;
  padding: 0 10px;
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
