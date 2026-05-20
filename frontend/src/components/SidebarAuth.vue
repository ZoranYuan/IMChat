<template>
  <aside class="rail">
    <div class="brand">
      <div class="brand-mark">SY</div>
      <div>
        <strong>SYCHAT</strong>
        <span>聊天与一起看视频</span>
      </div>
    </div>

    <section class="auth-panel">
      <div class="tabs">
        <button :class="{ active: authMode === 'login' }" @click="$emit('update:authMode', 'login')">登录</button>
        <button :class="{ active: authMode === 'register' }" @click="$emit('update:authMode', 'register')">注册</button>
      </div>
      <input :value="authForm.phone" placeholder="手机号" @input="authForm.phone = $event.target.value.trim()" />
      <input
        :value="authForm.password"
        placeholder="密码"
        type="password"
        @input="authForm.password = $event.target.value.trim()"
        @keydown.enter="$emit('submit-auth')"
      />
      <button class="primary wide" @click="$emit('submit-auth')">
        <LogIn :size="16" />
        {{ authMode === "login" ? "进入" : "创建账号" }}
      </button>
      <p class="muted small" v-if="currentUser.userId">当前用户：{{ currentUser.userId }}</p>
    </section>

    <section class="status-card">
      <div class="status-line">
        <span :class="['dot', wsConnected ? 'ok' : '']"></span>
        <span>{{ wsConnected ? "WebSocket 已连接" : "WebSocket 未连接" }}</span>
      </div>
      <button class="ghost wide" :disabled="!token" @click="$emit('connect-ws')">
        <RadioTower :size="16" />
        连接实时通道
      </button>
    </section>
  </aside>
</template>

<script setup>
import { LogIn, RadioTower } from "@lucide/vue";

defineProps({
  token: { type: String, default: "" },
  currentUser: { type: Object, required: true },
  authMode: { type: String, required: true },
  authForm: { type: Object, required: true },
  wsConnected: { type: Boolean, default: false },
});

defineEmits(["update:authMode", "submit-auth", "connect-ws"]);
</script>
