<template>
  <main class="login-page">
    <section class="login-hero">
      <div class="brand big">
        <div class="brand-mark">IM</div>
        <div>
          <strong>IM Watch</strong>
          <span>聊天与一起看视频</span>
        </div>
      </div>
      <h1>实时聊天，一起同步观影。</h1>
      <p>房间成员可以聊天、上传视频、同步播放进度，并把历史聊天作为弹幕回放。</p>
    </section>

    <section class="login-card">
      <div class="tabs">
        <button :class="{ active: authMode === 'login' }" @click="$emit('update:authMode', 'login')">登录</button>
        <button :class="{ active: authMode === 'register' }" @click="$emit('update:authMode', 'register')">注册</button>
      </div>

      <label>
        <span>手机号</span>
        <input :value="authForm.phone" placeholder="请输入手机号" @input="authForm.phone = $event.target.value.trim()" />
      </label>
      <label>
        <span>密码</span>
        <input
          :value="authForm.password"
          placeholder="请输入密码"
          type="password"
          @input="authForm.password = $event.target.value.trim()"
          @keydown.enter="$emit('submit-auth')"
        />
      </label>

      <button class="primary wide login-submit" @click="$emit('submit-auth')">
        <LogIn :size="17" />
        {{ authMode === "login" ? "登录工作台" : "创建账号" }}
      </button>
    </section>
  </main>
</template>

<script setup>
import { LogIn } from "@lucide/vue";

defineProps({
  authMode: { type: String, required: true },
  authForm: { type: Object, required: true },
});

defineEmits(["update:authMode", "submit-auth"]);
</script>
