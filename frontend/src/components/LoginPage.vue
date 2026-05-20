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
import { onBeforeUnmount, onMounted, ref } from "vue";
import { LogIn } from "@lucide/vue";
import githubIcon from "../assets/github.svg";
import wechatIcon from "../assets/wechat.svg";

defineProps({
  authMode: { type: String, required: true },
  authForm: { type: Object, required: true },
});

defineEmits(["update:authMode", "submit-auth", "oauth-login"]);

const titleText = "实时聊天，一起同步观影。";
const copyText = "房间成员可以聊天、上传视频、同步播放进度，并把历史聊天作为弹幕回放。";
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
