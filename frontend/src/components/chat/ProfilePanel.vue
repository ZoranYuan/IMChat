<script setup>
import { LogOut } from "@lucide/vue";

defineProps({ currentUser: { type: Object, required: true } });
defineEmits(["logout"]);
</script>

<template>
  <section class="flex h-full min-w-0 flex-col bg-white">
    <header class="px-5 pb-4 pt-6"><p class="mb-1 text-[10px] font-bold tracking-[0.16em] text-[#5b35f5]">ACCOUNT</p><h1 class="m-0 text-[22px] font-bold tracking-[-0.03em] text-[#17122c]">我的</h1></header>
    <div class="min-h-0 flex-1 overflow-y-auto px-5 pb-5">
      <div class="grid justify-items-center rounded-2xl border border-[#eeeaf7] bg-gradient-to-b from-[#faf9ff] to-white px-5 py-7 text-center">
        <span class="grid size-20 place-items-center overflow-hidden rounded-3xl text-2xl font-bold text-white shadow-[0_12px_28px_rgba(91,53,245,0.18)]" :style="{ backgroundColor: currentUser.avatarColor || '#5b35f5' }">
          <img v-if="currentUser.avatar" class="size-full object-cover" :src="currentUser.avatar" :alt="currentUser.nickName || currentUser.username" />
          <span v-else>{{ (currentUser.nickName || currentUser.name || currentUser.username || "我").slice(0, 1) }}</span>
        </span>
        <h2 class="mt-4 text-lg font-bold text-[#27213c]">{{ currentUser.nickName || currentUser.name || currentUser.username || "IM 用户" }}</h2>
        <p class="mt-1 text-xs text-[#aaa6b5]">@{{ currentUser.username || currentUser.userId || "user" }}</p>
        <span class="mt-3 inline-flex items-center gap-1.5 rounded-full bg-[#edf8f2] px-2.5 py-1 text-[11px] font-semibold text-[#42a977]"><i class="size-1.5 rounded-full bg-[#62c894]"></i>在线</span>
      </div>

      <div class="mt-4 grid grid-cols-2 divide-x divide-[#eeeaf7] rounded-xl border border-[#eeeaf7] bg-[#fcfbff] py-3 text-center">
        <span class="grid gap-1"><small class="text-[10px] text-[#aaa6b5]">手机</small><strong class="truncate px-2 text-[11px] font-semibold text-[#4e475f]">{{ currentUser.phone || "未设置" }}</strong></span>
        <span class="grid gap-1"><small class="text-[10px] text-[#aaa6b5]">用户 ID</small><strong class="truncate px-2 text-[11px] font-semibold text-[#4e475f]">{{ currentUser.userId || currentUser.id || "-" }}</strong></span>
      </div>

      <button class="mt-5 flex min-h-11 w-full items-center justify-center gap-2 rounded-xl border border-[#f2d6dc] bg-[#fff7f8] text-xs font-semibold text-[#c34d65] transition-colors hover:bg-[#fff0f3]" type="button" @click="$emit('logout')"><LogOut :size="18" />退出登录</button>
    </div>
  </section>
</template>
