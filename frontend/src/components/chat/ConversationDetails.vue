<script setup>
import { Bell, BellOff, FileText, Pin, PinOff, Search, Trash2, Users, X } from "@lucide/vue";
import { sharedFiles } from "../../mocks/chat.js";

defineProps({ conversation: { type: Object, default: null } });
defineEmits(["close", "toggle-pin", "toggle-mute", "clear"]);
</script>

<template>
  <aside v-if="conversation" class="details-panel">
    <header><strong>会话详情</strong><button class="icon-button" type="button" title="关闭" @click="$emit('close')"><X :size="19" /></button></header>
    <div class="details-scroll">
      <section class="details-identity">
        <span :style="{ backgroundColor: conversation.avatarColor }"><img v-if="conversation.avatar" :src="conversation.avatar" :alt="conversation.name" /><Users v-else-if="conversation.type === 'group'" :size="26" /><b v-else>{{ conversation.name.slice(0, 1) }}</b></span>
        <h3>{{ conversation.name }}</h3>
        <p>{{ conversation.type === "group" ? `${conversation.memberCount} 位成员` : conversation.online ? "当前在线" : "当前离线" }}</p>
      </section>

      <div class="detail-actions">
        <button type="button"><Search :size="18" /><span>搜索</span></button>
        <button type="button" @click="$emit('toggle-pin')"><PinOff v-if="conversation.pinned" :size="18" /><Pin v-else :size="18" /><span>{{ conversation.pinned ? "取消置顶" : "置顶" }}</span></button>
        <button type="button" @click="$emit('toggle-mute')"><Bell v-if="conversation.muted" :size="18" /><BellOff v-else :size="18" /><span>{{ conversation.muted ? "开启通知" : "消息免打扰" }}</span></button>
      </div>

      <section v-if="conversation.type === 'group'" class="details-section">
        <div class="details-title"><strong>群聊信息</strong><span>{{ conversation.memberCount }} 人</span></div>
        <p>{{ conversation.description || "用于团队日常沟通与项目协作。" }}</p>
      </section>

      <section class="details-section">
        <div class="details-title"><strong>共享文件</strong><span>{{ sharedFiles.length }}</span></div>
        <button v-for="file in sharedFiles" :key="file.id" class="shared-file" type="button"><FileText :size="19" /><span><strong>{{ file.name }}</strong><small>{{ file.meta }}</small></span></button>
      </section>

      <button class="danger-row" type="button" @click="$emit('clear')"><Trash2 :size="18" />清空聊天记录</button>
    </div>
  </aside>
</template>
