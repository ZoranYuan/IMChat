<template>
  <section class="friends-pane">
    <div class="pane-head">
      <div>
        <p class="eyebrow">Contacts</p>
        <h1>好友</h1>
      </div>
      <button class="icon-btn" :disabled="!token" title="刷新好友" @click="$emit('refresh')">
        <RefreshCcw :size="18" />
      </button>
    </div>

    <div class="friend-form">
      <input
        :value="friendForm.keyword"
        placeholder="用户名 / 手机号"
        @input="friendForm.keyword = $event.target.value.trim()"
      />
      <input :value="friendForm.message" placeholder="申请留言" @input="friendForm.message = $event.target.value" />
      <button class="primary wide" :disabled="!token" @click="$emit('submit-request')">
        <UserPlus :size="16" />
        发送好友申请
      </button>
    </div>

    <div class="friend-section">
      <div class="section-title">
        <span>好友列表</span>
        <span>{{ friends.length }}</span>
      </div>
      <button v-for="friend in friends" :key="friend.friendUserId" class="friend-item" @click="$emit('open-chat', friend)">
        <div class="avatar">{{ displayName(friend).slice(0, 2).toUpperCase() }}</div>
        <div class="item-main">
          <strong>{{ displayName(friend) }}</strong>
          <p>{{ friend.friendUsername || displayName(friend) }}</p>
        </div>
        <MessageCircle :size="17" />
      </button>
      <p v-if="friends.length === 0" class="empty-hint">暂无好友</p>
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
          <button class="ghost" @click="$emit('operate-request', request.requestId, 1)">同意</button>
          <button class="ghost" @click="$emit('operate-request', request.requestId, 2)">拒绝</button>
        </div>
      </article>
      <p v-if="pendingRequests.length === 0" class="empty-hint">暂无待处理申请</p>
    </div>
  </section>
</template>

<script setup>
import { computed } from "vue";
import { MessageCircle, RefreshCcw, UserPlus } from "@lucide/vue";

const props = defineProps({
  token: { type: String, default: "" },
  friends: { type: Array, default: () => [] },
  friendRequests: { type: Array, default: () => [] },
  friendForm: { type: Object, required: true },
  formatTime: { type: Function, required: true },
});

defineEmits(["refresh", "submit-request", "operate-request", "open-chat"]);

const pendingRequests = computed(() => props.friendRequests.filter((request) => request.status === 1));

function displayName(friend) {
  return friend.displayName || friend.friendUsername || "好友";
}
</script>
