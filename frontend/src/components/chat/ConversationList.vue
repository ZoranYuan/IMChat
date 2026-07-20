<script setup>
import { BellOff, Edit3, LoaderCircle, Pin, Plus, Search, Users } from "@lucide/vue";
import { computed, ref } from "vue";

const props = defineProps({
  conversations: { type: Array, default: () => [] },
  activeId: { type: String, default: "" },
  loading: { type: Boolean, default: false },
});

defineEmits(["select", "create-room"]);

const query = ref("");
const filter = ref("all");
const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  return props.conversations.filter((item) => {
    const matchKeyword = !keyword || item.name.toLowerCase().includes(keyword) || item.subtitle.toLowerCase().includes(keyword);
    const matchFilter = filter.value === "all" || (filter.value === "unread" && item.unread > 0) || (filter.value === "groups" && item.type === "group");
    return matchKeyword && matchFilter;
  });
});
</script>

<template>
  <section class="directory-panel">
    <header class="directory-header">
      <div>
        <p class="directory-kicker">SYCHAT</p>
        <h1>消息</h1>
      </div>
      <div class="header-actions">
        <button class="icon-button" type="button" title="新建群聊" @click="$emit('create-room')"><Plus :size="19" /></button>
        <button class="icon-button" type="button" title="发起会话"><Edit3 :size="18" /></button>
      </div>
    </header>

    <label class="search-box">
      <Search :size="17" />
      <input v-model="query" type="search" placeholder="搜索会话或消息" />
      <kbd>⌘ K</kbd>
    </label>

    <div class="segmented-control" aria-label="会话筛选">
      <button :class="{ active: filter === 'all' }" type="button" @click="filter = 'all'">全部</button>
      <button :class="{ active: filter === 'unread' }" type="button" @click="filter = 'unread'">未读</button>
      <button :class="{ active: filter === 'groups' }" type="button" @click="filter = 'groups'">群聊</button>
    </div>

    <div class="conversation-scroll">
      <div v-if="loading" class="inline-state"><LoaderCircle class="spin" :size="20" /><span>正在同步会话</span></div>
      <div v-else-if="filtered.length === 0" class="inline-state empty"><span>没有匹配的会话</span><small>尝试更换关键词或筛选条件</small></div>
      <button
        v-for="conversation in filtered"
        v-else
        :key="conversation.id"
        :class="['conversation-item', { active: conversation.id === activeId }]"
        type="button"
        @click="$emit('select', conversation.id)"
      >
        <span class="conversation-avatar" :style="{ backgroundColor: conversation.avatarColor }">
          <img v-if="conversation.avatar" :src="conversation.avatar" :alt="conversation.name" />
          <Users v-else-if="conversation.type === 'group'" :size="19" />
          <span v-else>{{ conversation.name.slice(0, 1) }}</span>
          <i v-if="conversation.online && conversation.type === 'direct'" class="online-dot"></i>
        </span>
        <span class="conversation-copy">
          <span class="conversation-line">
            <strong>{{ conversation.name }}</strong>
            <time>{{ conversation.time }}</time>
          </span>
          <span class="conversation-line preview">
            <span>{{ conversation.subtitle }}</span>
            <span class="item-signals">
              <Pin v-if="conversation.pinned" :size="12" />
              <BellOff v-if="conversation.muted" :size="12" />
              <b v-if="conversation.unread">{{ conversation.unread > 99 ? "99+" : conversation.unread }}</b>
            </span>
          </span>
        </span>
      </button>
    </div>
  </section>
</template>
