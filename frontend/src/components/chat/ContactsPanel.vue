<script setup>
import { Check, MessageCircle, Plus, Search, UserPlus, X } from "@lucide/vue";
import { computed, reactive, ref } from "vue";

const props = defineProps({
  contacts: { type: Array, default: () => [] },
  requests: { type: Array, default: () => [] },
});

const emit = defineEmits(["open-chat", "handle-request", "add-friend"]);
const query = ref("");
const showAdd = ref(false);
const adding = ref(false);
const addForm = reactive({ toUserId: "", message: "你好，我想加你为好友" });

const filteredContacts = computed(() => {
  const keyword = query.value.trim().toLowerCase();
  return props.contacts.filter((item) => !keyword || item.name.toLowerCase().includes(keyword) || item.username?.toLowerCase().includes(keyword));
});

const submitRequest = async () => {
  if (!addForm.toUserId.trim()) return;
  adding.value = true;
  try {
    await emit("add-friend", { ...addForm });
    showAdd.value = false;
    addForm.toUserId = "";
  } finally {
    adding.value = false;
  }
};
</script>

<template>
  <section class="directory-panel contacts-panel">
    <header class="directory-header">
      <div><p class="directory-kicker">CONTACTS</p><h1>联系人</h1></div>
      <button class="icon-button" type="button" title="添加好友" @click="showAdd = !showAdd"><UserPlus :size="19" /></button>
    </header>

    <label class="search-box"><Search :size="17" /><input v-model="query" type="search" placeholder="搜索联系人" /></label>

    <form v-if="showAdd" class="add-friend-form" @submit.prevent="submitRequest">
      <div class="form-title"><strong>添加好友</strong><button type="button" title="关闭" @click="showAdd = false"><X :size="17" /></button></div>
      <label><span>用户 ID</span><input v-model.trim="addForm.toUserId" required placeholder="输入对方的用户 ID" /></label>
      <label><span>申请留言</span><input v-model.trim="addForm.message" required maxlength="60" /></label>
      <button class="primary-button" type="submit" :disabled="adding">{{ adding ? "发送中..." : "发送申请" }}</button>
    </form>

    <div class="contact-scroll">
      <section v-if="requests.length" class="contact-section">
        <div class="section-label"><span>新的好友</span><b>{{ requests.length }}</b></div>
        <article v-for="request in requests" :key="request.id" class="request-card">
          <span class="contact-avatar" :style="{ backgroundColor: request.avatarColor }">{{ request.name.slice(0, 1) }}</span>
          <span class="request-copy"><strong>{{ request.name }}</strong><small>{{ request.note }}</small></span>
          <span class="request-actions">
            <button type="button" title="同意" @click="$emit('handle-request', request, true)"><Check :size="16" /></button>
            <button type="button" title="拒绝" @click="$emit('handle-request', request, false)"><X :size="16" /></button>
          </span>
        </article>
      </section>

      <section class="contact-section">
        <div class="section-label"><span>全部联系人</span><span>{{ filteredContacts.length }}</span></div>
        <button v-for="contact in filteredContacts" :key="contact.id" class="contact-item" type="button" @click="$emit('open-chat', contact)">
          <span class="contact-avatar" :style="{ backgroundColor: contact.avatarColor }">
            <img v-if="contact.avatar" :src="contact.avatar" :alt="contact.name" />
            <span v-else>{{ contact.name.slice(0, 1) }}</span>
            <i v-if="contact.online" class="online-dot"></i>
          </span>
          <span><strong>{{ contact.name }}</strong><small>{{ contact.role }}<template v-if="contact.username"> · {{ contact.username }}</template></small></span>
          <MessageCircle :size="17" />
        </button>
        <div v-if="!filteredContacts.length" class="inline-state empty"><span>没有找到联系人</span></div>
      </section>
    </div>
  </section>
</template>
