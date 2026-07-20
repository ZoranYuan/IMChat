<script setup>
import { LogOut, MessageCircle, Users, UserRound } from "@lucide/vue";

defineProps({
  activeSection: { type: String, required: true },
  currentUser: { type: Object, required: true },
  connection: { type: String, default: "disconnected" },
});

defineEmits(["select", "logout", "retry"]);
</script>

<template>
  <aside class="app-rail">
    <button class="rail-avatar" type="button" title="个人资料" @click="$emit('select', 'profile')">
      <img v-if="currentUser.avatar" :src="currentUser.avatar" :alt="currentUser.nickName || currentUser.name" />
      <span v-else>{{ (currentUser.nickName || currentUser.name || currentUser.username || "我").slice(0, 1) }}</span>
      <i :class="['presence-dot', connection]" aria-hidden="true"></i>
    </button>

    <nav class="rail-nav" aria-label="主要导航">
      <button :class="{ active: activeSection === 'conversations' }" type="button" title="消息" @click="$emit('select', 'conversations')">
        <MessageCircle :size="21" />
        <span>消息</span>
      </button>
      <button :class="{ active: activeSection === 'contacts' }" type="button" title="联系人" @click="$emit('select', 'contacts')">
        <Users :size="21" />
        <span>联系人</span>
      </button>
      <button :class="{ active: activeSection === 'profile' }" type="button" title="我的" @click="$emit('select', 'profile')">
        <UserRound :size="21" />
        <span>我的</span>
      </button>
    </nav>

    <div class="rail-bottom">
      <button class="connection-button" type="button" :title="connection === 'connected' ? '实时连接正常' : '点击重新连接'" @click="$emit('retry')">
        <i :class="['connection-indicator', connection]"></i>
      </button>
      <button type="button" title="退出登录" @click="$emit('logout')">
        <LogOut :size="20" />
        <span>退出</span>
      </button>
    </div>
  </aside>
</template>
