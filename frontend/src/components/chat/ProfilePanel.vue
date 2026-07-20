<script setup>
import { Bell, ChevronRight, CircleHelp, LogOut, ShieldCheck } from "@lucide/vue";

defineProps({ currentUser: { type: Object, required: true } });
defineEmits(["logout"]);
</script>

<template>
  <section class="directory-panel profile-panel">
    <header class="directory-header"><div><p class="directory-kicker">ACCOUNT</p><h1>我的</h1></div></header>
    <div class="profile-scroll">
      <div class="profile-card">
        <span class="profile-avatar" :style="{ backgroundColor: currentUser.avatarColor || '#2856a6' }">
          <img v-if="currentUser.avatar" :src="currentUser.avatar" :alt="currentUser.nickName || currentUser.username" />
          <span v-else>{{ (currentUser.nickName || currentUser.name || currentUser.username || "我").slice(0, 1) }}</span>
        </span>
        <h2>{{ currentUser.nickName || currentUser.name || currentUser.username || "IM 用户" }}</h2>
        <p>@{{ currentUser.username || currentUser.userId || "user" }}</p>
        <span class="profile-status"><i></i>在线</span>
      </div>

      <div class="profile-meta">
        <span><small>手机</small><strong>{{ currentUser.phone || "未设置" }}</strong></span>
        <span><small>用户 ID</small><strong>{{ currentUser.userId || currentUser.id || "-" }}</strong></span>
      </div>

      <div class="settings-list">
        <button type="button"><Bell :size="18" /><span>消息通知</span><ChevronRight :size="17" /></button>
        <button type="button"><ShieldCheck :size="18" /><span>账号与安全</span><ChevronRight :size="17" /></button>
        <button type="button"><CircleHelp :size="18" /><span>帮助与反馈</span><ChevronRight :size="17" /></button>
      </div>

      <button class="logout-button" type="button" @click="$emit('logout')"><LogOut :size="18" />退出登录</button>
    </div>
  </section>
</template>
