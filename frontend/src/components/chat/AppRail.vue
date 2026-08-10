<script setup>
import { CalendarDays, CheckSquare, Folder, LogOut, MessageCircle, Settings, Users, UsersRound } from "@lucide/vue";

defineProps({
  activeSection: { type: String, required: true },
  currentUser: { type: Object, required: true },
  connection: { type: String, default: "disconnected" },
});

defineEmits(["select", "logout", "retry", "unsupported"]);
</script>

<template>
  <aside class="hidden h-full w-[164px] flex-col items-center bg-white px-3 py-[18px] text-[#98a2b3] shadow-[4px_0_18px_rgba(54,42,116,0.04)] min-[768px]:flex">
    <button class="relative grid size-[42px] place-items-center rounded-xl bg-gradient-to-br from-[#6f4cff] to-[#4424df] text-[15px] font-bold text-white shadow-[0_10px_22px_rgba(91,53,245,0.24)] transition-transform duration-150 hover:-translate-y-0.5"
      type="button" title="个人资料" @click="$emit('select', 'profile')">
      <img v-if="currentUser.avatar" class="size-full rounded-xl object-cover" :src="currentUser.avatar" :alt="currentUser.nickName || currentUser.name" />
      <span v-else>{{ (currentUser.nickName || currentUser.name || currentUser.username || "我").slice(0, 1) }}</span>
      <i :class="[
        'absolute -bottom-0.5 -right-0.5 size-3 rounded-full border-2 border-white',
        connection === 'connected' ? 'bg-[#62c894]' : connection === 'connecting' ? 'bg-[#d29a42]' : 'bg-[#aeb8c5]'
      ]" aria-hidden="true"></i>
    </button>

    <nav class="mt-9 flex w-full flex-1 flex-col gap-1" aria-label="主要导航">
      <button :class="activeSection === 'conversations'
        ? 'bg-gradient-to-br from-[#6d4aff] to-[#4a29e5] text-white shadow-[0_12px_24px_rgba(91,53,245,0.22)]'
        : 'text-[#8d91a3] hover:bg-[#f1efff] hover:text-[#5b35f5]'"
        class="flex min-h-[42px] flex-col items-center justify-center gap-1 rounded-[9px] text-[11px] transition-[background,color,box-shadow] duration-150"
        type="button" title="消息" @click="$emit('select', 'conversations')">
        <MessageCircle :size="21" /><span>会话</span>
      </button>
      <button :class="activeSection === 'contacts'
        ? 'bg-gradient-to-br from-[#6d4aff] to-[#4a29e5] text-white shadow-[0_12px_24px_rgba(91,53,245,0.22)]'
        : 'text-[#8d91a3] hover:bg-[#f1efff] hover:text-[#5b35f5]'"
        class="flex min-h-[42px] flex-col items-center justify-center gap-1 rounded-[9px] text-[11px] transition-[background,color,box-shadow] duration-150"
        type="button" title="联系人" @click="$emit('select', 'contacts')">
        <Users :size="21" /><span>通讯录</span>
      </button>
      <button v-for="item in [
        { label: '群组', icon: UsersRound },
        { label: '文件', icon: Folder },
        { label: '日历', icon: CalendarDays },
        { label: '待办', icon: CheckSquare },
      ]" :key="item.label" class="flex min-h-[42px] flex-col items-center justify-center gap-1 rounded-[9px] text-[11px] text-[#8d91a3] transition-[background,color] duration-150 hover:bg-[#f1efff] hover:text-[#5b35f5]"
        type="button" :title="item.label" @click="$emit('unsupported', item.label)">
        <component :is="item.icon" :size="21" /><span>{{ item.label }}</span>
      </button>
      <button :class="activeSection === 'profile'
        ? 'bg-gradient-to-br from-[#6d4aff] to-[#4a29e5] text-white shadow-[0_12px_24px_rgba(91,53,245,0.22)]'
        : 'text-[#8d91a3] hover:bg-[#f1efff] hover:text-[#5b35f5]'"
        class="flex min-h-[42px] flex-col items-center justify-center gap-1 rounded-[9px] text-[11px] transition-[background,color,box-shadow] duration-150"
        type="button" title="我的" @click="$emit('select', 'profile')">
        <Settings :size="21" /><span>设置</span>
      </button>
    </nav>

    <div class="mt-auto flex w-full flex-col gap-1">
      <button class="flex min-h-[42px] flex-col items-center justify-center gap-1 rounded-[9px] text-[11px] text-[#8d91a3] transition-colors duration-150 hover:bg-[#f1efff] hover:text-[#5b35f5]"
        type="button" :title="connection === 'connected' ? '实时连接正常' : '点击重新连接'" @click="$emit('retry')">
        <i :class="[
          'size-2.5 rounded-full',
          connection === 'connected' ? 'bg-[#62c894] shadow-[0_0_0_4px_rgba(98,200,148,0.14)]' : connection === 'connecting' ? 'bg-[#d29a42]' : 'bg-[#dc6570]'
        ]"></i>
        <span>{{ connection === 'connected' ? '在线' : '连接' }}</span>
      </button>
      <button class="flex min-h-[42px] flex-col items-center justify-center gap-1 rounded-[9px] text-[11px] text-[#8d91a3] transition-colors duration-150 hover:bg-[#fff0f3] hover:text-[#dc6570]"
        type="button" title="退出登录" @click="$emit('logout')">
        <LogOut :size="20" /><span>退出</span>
      </button>
    </div>
  </aside>
</template>
