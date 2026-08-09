<script setup>
import { Eye, EyeOff, Globe2, LockKeyhole, MessageCircle, Phone, QrCode } from "@lucide/vue";
import { computed, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useChatStore } from "../stores/chat.js";
import { messageTips } from "../utils/messageTips.js";

const route = useRoute();
const router = useRouter();
const { authenticate } = useChatStore();

const mode = ref("login");
const showPassword = ref(false);
const submitting = ref(false);
const form = reactive({ account: "", password: "", reconfirmPassword: "", remember: true });
const errors = reactive({ account: "", password: "", reconfirmPassword: "", submit: "" });

const title = computed(() => mode.value === "login" ? "欢迎回来" : "创建你的账号");
const validate = () => {
  errors.account = mode.value === "register"
    ? (/^1\d{10}$/.test(form.account) ? "" : "注册请输入 11 位手机号")
    : (/^1\d{10}$/.test(form.account) ? "" : "请输入 11 位手机号");
  errors.password = form.password.length >= 6 ? "" : "密码至少需要 6 位";
  errors.reconfirmPassword = mode.value === "register" && form.reconfirmPassword !== form.password ? "两次输入的密码不一致" : "";
  errors.submit = "";
  return !errors.account && !errors.password && !errors.reconfirmPassword;
};

const submit = async () => {
  if (!validate()) return;
  submitting.value = true;
  try {
    await authenticate({ mode: mode.value, ...form });
    if (mode.value === "register") {
      messageTips.success("注册成功，请登录");
      mode.value = "login";
      form.password = "";
      form.reconfirmPassword = "";
      return;
    }
    messageTips.success("登录成功");
    router.replace(typeof route.query.redirect === "string" ? route.query.redirect : "/chat");
  } catch (error) {
    errors.submit = error.message;
    messageTips.error({ title: mode.value === "login" ? "登录失败" : "注册失败", message: error.message });
  } finally {
    submitting.value = false;
  }
};

watch(mode, () => {
  errors.account = "";
  errors.password = "";
  errors.reconfirmPassword = "";
  errors.submit = "";
});
</script>

<template>
  <main class="auth-page">
    <section class="auth-brand-panel">
      <div class="auth-brand"><span>
          <MessageCircle :size="24" />
        </span>
        <div><strong>IMChat</strong></div>
      </div>
      <div class="auth-preview">
        <span class="preview-card-dot"></span>
        <span class="preview-card-dot"></span>
        <div class="preview-screen"><i></i><i></i><i></i>
          <p></p>
          <p></p><b></b>
        </div>
        <div class="preview-chat-bubble"><span></span><span></span><span></span></div>
      </div>
    </section>

    <section class="auth-form-panel">
      <div class="mobile-auth-brand"><span>
          <MessageCircle :size="30" />
        </span><strong>IMChat</strong></div>
      <div class="auth-form-wrap">
        <header><span class="auth-card-logo">
            <MessageCircle :size="30" />
          </span>
          <h2>{{ title }}</h2>
          <p>{{ mode === "login" ? "请登录您的账号" : "使用手机号注册一个新账号" }}</p>
        </header>

        <div class="auth-tabs">
          <button :class="{ active: mode === 'login' }" type="button" @click="mode = 'login'">登录</button>
          <button :class="{ active: mode === 'register' }" type="button" @click="mode = 'register'">注册</button>
        </div>

        <form @submit.prevent="submit">
          <label class="auth-field"><span>账号</span>
            <div>
              <Phone :size="18" /><input v-model.trim="form.account" inputmode="tel" autocomplete="tel" maxlength="11"
                placeholder="请输入手机号" />
            </div><small v-if="errors.account">{{ errors.account }}</small>
          </label>
          <label class="auth-field"><span>密码</span>
            <div>
              <LockKeyhole :size="18" /><input v-model="form.password" :type="showPassword ? 'text' : 'password'"
                :autocomplete="mode === 'login' ? 'current-password' : 'new-password'" placeholder="请输入密码" /><button
                type="button" :title="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword">
                <EyeOff v-if="showPassword" :size="18" />
                <Eye v-else :size="18" />
              </button>
            </div><small v-if="errors.password">{{ errors.password }}</small>
          </label>
          <label v-if="mode === 'register'" class="auth-field"><span>确认密码</span>
            <div>
              <LockKeyhole :size="18" /><input v-model="form.reconfirmPassword"
                :type="showPassword ? 'text' : 'password'" autocomplete="new-password" placeholder="请再次输入密码" />
            </div><small v-if="errors.reconfirmPassword">{{ errors.reconfirmPassword }}</small>
          </label>

          <div v-if="mode === 'login'" class="auth-options"><label><input v-model="form.remember"
                type="checkbox" />记住我</label><button type="button"
              @click="messageTips.info('暂未提供找回密码接口')">忘记密码?</button></div>
          <div v-if="errors.submit" class="auth-error">{{ errors.submit }}</div>
          <button class="auth-submit" type="submit" :disabled="submitting">{{ submitting ? "正在提交..." : mode === "login"
            ? "登录" : "注册" }}</button>
        </form>

        <div class="auth-social"><span>其他登录方式</span>
          <div><button type="button" title="微信登录" @click="messageTips.info('暂未提供第三方登录接口')">
              <MessageCircle :size="20" />
            </button><button type="button" title="GitHub 登录" @click="messageTips.info('暂未提供第三方登录接口')">
              <Globe2 :size="20" />
            </button><button type="button" title="扫码登录" @click="messageTips.info('暂未提供扫码登录接口')">
              <QrCode :size="20" />
            </button></div>
        </div>
        <p class="auth-terms">{{ mode === "login" ? "还没有账号？" : "已有账号？" }} <button type="button"
            @click="mode = mode === 'login' ? 'register' : 'login'">{{ mode === "login" ? "立即注册" : "返回登录" }}</button>
        </p>
      </div>
    </section>
  </main>
</template>
