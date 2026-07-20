<script setup>
import { CheckCircle2, Eye, EyeOff, LockKeyhole, MessageCircle, Phone, Users } from "@lucide/vue";
import { computed, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useChatStore } from "../composables/useChatStore.js";
import { messageTips } from "../utils/messageTips.js";

const route = useRoute();
const router = useRouter();
const { authenticate } = useChatStore();

const mode = ref("login");
const showPassword = ref(false);
const submitting = ref(false);
const form = reactive({ phone: "", password: "", reconfirmPassword: "", remember: true });
const errors = reactive({ phone: "", password: "", reconfirmPassword: "", submit: "" });

const title = computed(() => mode.value === "login" ? "欢迎回来" : "创建你的账号");
const validate = () => {
  errors.phone = /^1\d{10}$/.test(form.phone) ? "" : "请输入 11 位手机号";
  errors.password = form.password.length >= 6 ? "" : "密码至少需要 6 位";
  errors.reconfirmPassword = mode.value === "register" && form.reconfirmPassword !== form.password ? "两次输入的密码不一致" : "";
  errors.submit = "";
  return !errors.phone && !errors.password && !errors.reconfirmPassword;
};

const submit = async () => {
  if (!validate()) return;
  submitting.value = true;
  try {
    await authenticate({ mode: mode.value, ...form });
    messageTips.success(mode.value === "login" ? "登录成功" : "注册成功，已自动登录");
    router.replace(typeof route.query.redirect === "string" ? route.query.redirect : "/chat");
  } catch (error) {
    errors.submit = error.message;
    messageTips.error({ title: mode.value === "login" ? "登录失败" : "注册失败", message: error.message });
  } finally {
    submitting.value = false;
  }
};

watch(mode, () => {
  errors.phone = "";
  errors.password = "";
  errors.reconfirmPassword = "";
  errors.submit = "";
});
</script>

<template>
  <main class="auth-page">
    <section class="auth-brand-panel">
      <div class="auth-brand"><span><MessageCircle :size="24" /></span><div><strong>SYCHAT</strong><small>专注沟通，高效协作</small></div></div>
      <div class="auth-copy">
        <span class="auth-eyebrow">TEAM MESSAGING</span>
        <h1>让每一次沟通<br />都有清晰上下文</h1>
        <p>实时消息、好友与群组协作集中在一处，适配桌面和移动设备。</p>
      </div>
      <div class="auth-preview">
        <div class="preview-top"><span class="preview-avatar">林</span><div><strong>IM 产品共创组</strong><small><i></i>8 位成员在线</small></div></div>
        <div class="preview-message"><span>林岚</span><p>交互稿已经更新，可以开始联调了。</p><time>10:42</time></div>
        <div class="preview-message mine"><p>收到，我会先检查移动端适配。</p><CheckCircle2 :size="14" /></div>
      </div>
      <div class="auth-trust"><Users :size="17" /><span>为团队内部沟通提供稳定的实时连接</span></div>
    </section>

    <section class="auth-form-panel">
      <div class="mobile-auth-brand"><span><MessageCircle :size="22" /></span><strong>SYCHAT</strong></div>
      <div class="auth-form-wrap">
        <header><h2>{{ title }}</h2><p>{{ mode === "login" ? "登录后继续处理你的会话" : "使用手机号注册一个新账号" }}</p></header>

        <div class="auth-tabs">
          <button :class="{ active: mode === 'login' }" type="button" @click="mode = 'login'">登录</button>
          <button :class="{ active: mode === 'register' }" type="button" @click="mode = 'register'">注册</button>
        </div>

        <form @submit.prevent="submit">
          <label class="auth-field"><span>手机号</span><div><Phone :size="18" /><input v-model.trim="form.phone" inputmode="tel" autocomplete="tel" maxlength="11" placeholder="请输入手机号" /></div><small v-if="errors.phone">{{ errors.phone }}</small></label>
          <label class="auth-field"><span>密码</span><div><LockKeyhole :size="18" /><input v-model="form.password" :type="showPassword ? 'text' : 'password'" :autocomplete="mode === 'login' ? 'current-password' : 'new-password'" placeholder="请输入密码" /><button type="button" :title="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword"><EyeOff v-if="showPassword" :size="18" /><Eye v-else :size="18" /></button></div><small v-if="errors.password">{{ errors.password }}</small></label>
          <label v-if="mode === 'register'" class="auth-field"><span>确认密码</span><div><LockKeyhole :size="18" /><input v-model="form.reconfirmPassword" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" placeholder="请再次输入密码" /></div><small v-if="errors.reconfirmPassword">{{ errors.reconfirmPassword }}</small></label>

          <div v-if="mode === 'login'" class="auth-options"><label><input v-model="form.remember" type="checkbox" />记住登录状态</label><button type="button" @click="messageTips.info('暂未提供找回密码接口')">忘记密码</button></div>
          <div v-if="errors.submit" class="auth-error">{{ errors.submit }}</div>
          <button class="auth-submit" type="submit" :disabled="submitting">{{ submitting ? "正在提交..." : mode === "login" ? "登录" : "注册并登录" }}</button>
        </form>

        <p class="auth-terms">继续操作即表示你同意服务条款和隐私政策</p>
      </div>
    </section>
  </main>
</template>
