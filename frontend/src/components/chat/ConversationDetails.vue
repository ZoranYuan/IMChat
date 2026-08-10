<script setup>
import { Bell, BellOff, FileText, Pin, PinOff, Search, Trash2, Users, X } from "@lucide/vue";
import { sharedFiles } from "../../mocks/chat.js";

defineProps({ conversation: { type: Object, default: null } });
defineEmits(["close", "toggle-pin", "toggle-mute", "clear"]);
</script>

<template>
  <aside v-if="conversation" class="details-panel col-[5] flex min-w-0 w-full flex-col overflow-hidden bg-white max-[767px]:fixed max-[767px]:inset-0 max-[767px]:z-[45]">
    <header class="flex min-h-[68px] items-center justify-between border-b border-[#ebe9f7] px-4 text-[13px] font-semibold"><strong>会话详情</strong><button class="grid size-9 place-items-center rounded-lg bg-transparent text-[#9893a6] hover:bg-[#f8f7ff] hover:text-[#5b35f5]" type="button" title="关闭" @click="$emit('close')"><X :size="19" /></button></header>
    <div class="min-h-0 overflow-auto px-[18px] py-[22px]">
      <section class="flex flex-col items-center border-b border-[#ebe9f7] pb-[22px] text-center">
        <span class="grid size-16 place-items-center overflow-hidden rounded-[13px] text-2xl font-bold text-white" :style="{ backgroundColor: conversation.avatarColor }"><img v-if="conversation.avatar" class="size-full object-cover" :src="conversation.avatar" :alt="conversation.name" /><Users v-else-if="conversation.type === 'group'" :size="26" /><b v-else>{{ conversation.name.slice(0, 1) }}</b></span>
        <h3 class="mb-1 mt-[13px] text-base font-bold">{{ conversation.name }}</h3>
        <p class="m-0 text-[10px] text-[#8d889d]">{{ conversation.type === "group" ? `${conversation.memberCount} 位成员` : conversation.online ? "当前在线" : "当前离线" }}</p>
      </section>

      <div class="grid grid-cols-3 gap-1.5 border-b border-[#ebe9f7] py-[18px]">
        <button class="flex min-h-[58px] flex-col items-center justify-center gap-1.5 rounded-lg bg-[#f8f7ff] text-[9px] text-[#7c7891] hover:bg-[#f1efff] hover:text-[#5b35f5]" type="button"><Search :size="18" /><span>搜索</span></button>
        <button class="flex min-h-[58px] flex-col items-center justify-center gap-1.5 rounded-lg bg-[#f8f7ff] text-[9px] text-[#7c7891] hover:bg-[#f1efff] hover:text-[#5b35f5]" type="button" @click="$emit('toggle-pin')"><PinOff v-if="conversation.pinned" :size="18" /><Pin v-else :size="18" /><span>{{ conversation.pinned ? "取消置顶" : "置顶" }}</span></button>
        <button class="flex min-h-[58px] flex-col items-center justify-center gap-1.5 rounded-lg bg-[#f8f7ff] text-[9px] text-[#7c7891] hover:bg-[#f1efff] hover:text-[#5b35f5]" type="button" @click="$emit('toggle-mute')"><Bell v-if="conversation.muted" :size="18" /><BellOff v-else :size="18" /><span>{{ conversation.muted ? "开启通知" : "消息免打扰" }}</span></button>
      </div>

      <section v-if="conversation.type === 'group'" class="border-b border-[#ebe9f7] py-[18px]">
        <div class="mb-3 flex items-center justify-between"><strong class="text-xs">群聊信息</strong><span class="text-[10px] text-[#aaa6b5]">{{ conversation.memberCount }} 人</span></div>
        <p class="m-0 text-[11px] leading-[1.7] text-[#7c7891]">{{ conversation.description || "用于团队日常沟通与项目协作。" }}</p>
      </section>

      <section class="border-b border-[#ebe9f7] py-[18px]">
        <div class="mb-3 flex items-center justify-between"><strong class="text-xs">共享文件</strong><span class="text-[10px] text-[#aaa6b5]">{{ sharedFiles.length }}</span></div>
        <button v-for="file in sharedFiles" :key="file.id" class="flex w-full items-center gap-2.5 rounded-lg px-1.5 py-2 text-left text-[#5b35f5] hover:bg-[#f8f7ff]" type="button"><FileText :size="19" /><span class="grid min-w-0 gap-0.5"><strong class="truncate text-[10px] text-[#332d45]">{{ file.name }}</strong><small class="text-[9px] text-[#aaa6b5]">{{ file.meta }}</small></span></button>
      </section>

      <button class="mt-4 flex min-h-11 w-full items-center gap-2 rounded-lg bg-[#fff5f6] px-3 text-[11px] text-[#c0444d] hover:bg-[#ffeff1]" type="button" @click="$emit('clear')"><Trash2 :size="18" />清空聊天记录</button>
    </div>
  </aside>
</template>
