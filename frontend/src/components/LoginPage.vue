<template>
  <main class="login-page">
    <button class="login-theme-toggle" type="button" :title="themeLabel" @click="$emit('toggle-theme')">
      <SunMedium v-if="themeMode === 'dark'" :size="16" />
      <MoonStar v-else :size="16" />
    </button>

    <section class="login-hero">
      <div class="brand big">
        <div class="brand-mark">SY</div>
        <div>
          <strong>SYCHAT</strong>
          <span>聊天与好友</span>
        </div>
      </div>
      <h1 class="stream-title">
        {{ streamedTitle }}
        <span v-if="isStreamingTitle" class="stream-cursor"></span>
      </h1>
      <p class="stream-copy">
        {{ streamedCopy }}
        <span v-if="!isStreamingTitle" class="stream-cursor"></span>
      </p>
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
        <input :value="authForm.password" placeholder="请输入密码" type="password"
          @input="authForm.password = $event.target.value.trim()" @keydown.enter="$emit('submit-auth')" />
      </label>

      <button class="primary wide login-submit" @click="$emit('submit-auth')">
        <LogIn :size="17" />
        {{ authMode === "login" ? "登录" : "注册" }}
      </button>

      <div class="oauth-divider">
        <span></span>
        <em>第三方登录</em>
        <span></span>
      </div>

      <div class="oauth-actions">
        <button class="oauth-btn wechat" @click="$emit('oauth-login', 'wechat')">
          <img :src="wechatIcon" alt="" />
          微信登录
        </button>
        <button class="oauth-btn github" @click="$emit('oauth-login', 'github')">
          <img :src="githubIcon" alt="" />
          GitHub 登录
        </button>
      </div>
    </section>
  </main>
</template>

<script setup>
import { LogIn, MoonStar, SunMedium } from "@lucide/vue";
import { onBeforeUnmount, onMounted, ref } from "vue";
import githubIcon from "../assets/github.svg";
import wechatIcon from "../assets/wechat.svg";

defineProps({
  authMode: { type: String, required: true },
  authForm: { type: Object, required: true },
  themeMode: { type: String, required: true },
  themeLabel: { type: String, required: true },
});

defineEmits(["update:authMode", "submit-auth", "oauth-login", "toggle-theme"]);

const titleText = "实时聊天，一起同步观影。";
const sizeTitleText = "房间成员可以聊天、上传视频、同步播放进度，并把历史聊天作为弹幕回放。";
const streamedTitle = ref("");
const streamedCopy = ref("");
const isStreamingTitle = ref(true);
let streamTimer = 0;

function streamText(source, target, done, speed) {
  let index = 0;
  streamTimer = window.setInterval(() => {
    target.value = source.slice(0, index + 1);
    index += 1;
    if (index >= source.length) {
      window.clearInterval(streamTimer);
      done?.();
    }
  }, speed);
}

onMounted(() => {
  streamText(
    titleText,
    streamedTitle,
    () => {
      isStreamingTitle.value = false;
      window.setTimeout(() => streamText(copyText, streamedCopy, null, 34), 260);
    },
    58,
  );
});

onBeforeUnmount(() => {
  window.clearInterval(streamTimer);
});
</script>

<style scoped>
.login-page {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(360px, 420px);
  gap: clamp(28px, 4vw, 64px);
  align-items: center;
  min-height: 100vh;
  padding: clamp(28px, 5vw, 72px);
}

.login-theme-toggle {
  position: absolute;
  top: 22px;
  right: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 0;
  background: var(--surface-strong);
  color: var(--text);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(18px);
}

.login-hero {
  max-width: 760px;
}

.brand.big {
  margin-bottom: 40px;
}

.brand.big .brand-mark {
  width: 58px;
  height: 58px;
  border-radius: 18px;
  font-size: 18px;
}

.brand.big strong {
  font-size: 18px;
  letter-spacing: 0.08em;
}

.brand.big span {
  margin-top: 2px;
  font-size: 13px;
}

.login-hero h1 {
  max-width: 680px;
  min-height: 118px;
  margin: 0;
  font-size: clamp(42px, 5vw, 68px);
  line-height: 1.02;
  letter-spacing: -0.04em;
}

.login-hero p {
  max-width: 560px;
  min-height: 60px;
  margin-top: 18px;
  color: var(--muted);
  font-size: 15px;
  line-height: 1.8;
}

.stream-title,
.stream-copy {
  white-space: pre-wrap;
}

.stream-cursor {
  display: inline-block;
  width: 0.6em;
  height: 1em;
  margin-left: 4px;
  border-right: 2px solid var(--primary);
  transform: translateY(2px);
  animation: cursorBlink 0.88s step-end infinite;
}

@keyframes cursorBlink {
  50% {
    opacity: 0;
  }
}

.login-card {
  display: grid;
  gap: 16px;
  border: 1px solid var(--border);
  border-radius: var(--radius-xl);
  padding: 28px;
  background: var(--surface-strong);
  box-shadow: var(--shadow-lg);
  backdrop-filter: blur(22px);
}

.login-card label {
  display: grid;
  gap: 8px;
  color: var(--muted);
  font-size: 13px;
}

.login-submit {
  margin-top: 4px;
}

.oauth-divider {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 12px;
  color: var(--muted);
  font-size: 12px;
}

.oauth-divider span {
  height: 1px;
  background: var(--border);
}

.oauth-divider em {
  font-style: normal;
}

.oauth-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.oauth-btn {
  height: 44px;
  border-radius: var(--radius-sm);
  background: var(--surface-soft);
  color: var(--text);
}

.oauth-btn img {
  width: 18px;
  height: 18px;
  flex: 0 0 auto;
}

.oauth-btn.wechat img {
  filter: drop-shadow(0 0 12px rgba(7, 193, 96, 0.28));
}

.oauth-btn.github {
  color: var(--text);
}

:global(html[data-theme="light"]) .oauth-btn.github {
  color: #24292f;
}

:global(html[data-theme="dark"]) .oauth-btn.github {
  color: #f4f7ff;
}

@media (max-width: 980px) {
  .login-page {
    grid-template-columns: 1fr;
    align-content: start;
    padding-top: 76px;
  }

  .login-hero h1 {
    min-height: auto;
  }

  .login-theme-toggle {
    top: 12px;
    right: 12px;
    width: 38px;
    height: 38px;
  }
}
</style>
