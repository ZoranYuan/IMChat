<script setup>
import { MessageCircle, Plus, UserRound, Users, X } from "@lucide/vue";
import { onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import AppRail from "../components/chat/AppRail.vue";
import ChatPanel from "../components/chat/ChatPanel.vue";
import ContactsPanel from "../components/chat/ContactsPanel.vue";
import ConversationDetails from "../components/chat/ConversationDetails.vue";
import ConversationList from "../components/chat/ConversationList.vue";
import ProfilePanel from "../components/chat/ProfilePanel.vue";
import ConfirmDialog from "../components/common/ConfirmDialog.vue";
import { useChatStore } from "../composables/useChatStore.js";
import { useResponsive } from "../composables/useResponsive.js";
import { messageTips } from "../utils/messageTips.js";

const router = useRouter();
const { isMobile } = useResponsive();
const {
  state,
  activeConversation,
  activeMessages,
  loadWorkspace,
  selectConversation,
  loadOlderMessages,
  sendMessage,
  sendAttachment,
  sendReadAck,
  togglePinned,
  toggleMuted,
  clearConversation,
  handleFriendRequest,
  addFriendRequest,
  startConversation,
  createChatRoom,
  joinChatRoom,
  retryConnection,
  uploadStatus,
  uploadProgress,
  pauseUpload,
  resumeUpload,
  cancelUpload,
  logout,
} = useChatStore();

const activeSection = ref("conversations");
const mobileChatOpen = ref(false);
const detailsOpen = ref(false);
const roomDialogOpen = ref(false);
const roomMode = ref("create");
const roomSubmitting = ref(false);
const clearConfirmOpen = ref(false);
const loadError = ref("");
const roomForm = reactive({ roomName: "", description: "", avatar: "", inviteCode: "" });

const initialize = async () => {
  loadError.value = "";
  try {
    await loadWorkspace();
    if (state.activeConversationId) await selectConversation(state.activeConversationId);
    sendReadAck();
  } catch (error) {
    loadError.value = error.message;
    messageTips.error({ title: "数据加载失败", message: error.message });
  }
};

const chooseSection = (section) => {
  activeSection.value = section;
  mobileChatOpen.value = false;
  detailsOpen.value = false;
};

const chooseConversation = async (id) => {
  try {
    await selectConversation(id);
    mobileChatOpen.value = true;
    sendReadAck();
  } catch (error) {
    messageTips.error({ title: "消息加载失败", message: error.message });
  }
};

const handleSend = (text) => {
  try {
    sendMessage(text);
  } catch (error) {
    messageTips.warning(error.message);
  }
};

const handleAttachment = async (file, cType) => {
  try {
    await sendAttachment(file, cType);
    messageTips.success(cType === 2 ? "图片已发送" : cType === 3 ? "视频已发送" : "文件已发送");
  } catch (error) {
    messageTips.error({ title: "附件发送失败", message: error.message });
  }
};

const handleContact = async (contact) => {
  try {
    const conversation = await startConversation(contact);
    activeSection.value = "conversations";
    mobileChatOpen.value = true;
    return conversation;
  } catch (error) {
    messageTips.info(error.message);
  }
};

const handleRequest = async (request, accepted) => {
  try {
    await handleFriendRequest(request, accepted);
    messageTips.success(accepted ? "已同意好友申请" : "已拒绝好友申请");
  } catch (error) {
    messageTips.error(error.message);
  }
};

const handleAddFriend = async (form) => {
  try {
    await addFriendRequest(form);
    messageTips.success("好友申请已发送");
  } catch (error) {
    messageTips.error(error.message);
  }
};

const submitRoom = async () => {
  roomSubmitting.value = true;
  try {
    if (roomMode.value === "create") {
      if (!roomForm.roomName.trim()) throw new Error("请输入群聊名称。" );
      await createChatRoom(roomForm);
      messageTips.success("群聊创建成功");
    } else {
      if (!roomForm.inviteCode.trim()) throw new Error("请输入邀请码。" );
      await joinChatRoom(roomForm.inviteCode);
      messageTips.success("已加入群聊");
    }
    roomDialogOpen.value = false;
  } catch (error) {
    messageTips.error(error.message);
  } finally {
    roomSubmitting.value = false;
  }
};

const confirmClear = () => {
  clearConversation(activeConversation.value.id);
  clearConfirmOpen.value = false;
  detailsOpen.value = false;
  messageTips.success("本地聊天记录已清空");
};

const handleLogout = async () => {
  await logout();
  messageTips.info("已退出登录");
  router.replace("/login");
};

onMounted(initialize);
</script>

<template>
  <main class="im-app">
    <AppRail :active-section="activeSection" :current-user="state.currentUser" :connection="state.connection" @select="chooseSection" @retry="retryConnection" @logout="handleLogout" />

    <section :class="['im-workspace', { 'details-visible': detailsOpen }]">
      <aside :class="['directory-shell', { 'mobile-hidden': mobileChatOpen }]">
        <ConversationList v-if="activeSection === 'conversations'" :conversations="state.conversations" :active-id="state.activeConversationId" :loading="state.loading" @select="chooseConversation" @create-room="roomDialogOpen = true" />
        <ContactsPanel v-else-if="activeSection === 'contacts'" :contacts="state.contacts" :requests="state.friendRequests" @open-chat="handleContact" @handle-request="handleRequest" @add-friend="handleAddFriend" />
        <ProfilePanel v-else :current-user="state.currentUser" @logout="handleLogout" />
        <div v-if="loadError" class="directory-error"><strong>暂时无法同步数据</strong><span>{{ loadError }}</span><button type="button" @click="initialize">重新加载</button></div>
      </aside>

      <ChatPanel
        :class="{ 'mobile-visible': mobileChatOpen }"
        :conversation="activeConversation"
        :messages="activeMessages"
        :current-user="state.currentUser"
        :connection="state.connection"
        :loading="state.historyLoading"
        :has-more="state.historyHasMore[state.activeConversationId]"
        :mobile="isMobile"
        :upload-status="uploadStatus"
        :upload-progress="uploadProgress"
        @back="mobileChatOpen = false"
        @details="detailsOpen = true"
        @send="handleSend"
        @attachment="handleAttachment"
        @pause-upload="pauseUpload"
        @resume-upload="resumeUpload"
        @cancel-upload="cancelUpload"
        @load-older="loadOlderMessages"
        @unsupported="(name) => messageTips.info(`${name}功能待对应接口完善后接入`)"
      />

      <div v-if="detailsOpen" class="details-backdrop" @click="detailsOpen = false"></div>
      <ConversationDetails :class="{ open: detailsOpen }" :conversation="activeConversation" @close="detailsOpen = false" @toggle-pin="messageTips.success(togglePinned(activeConversation.id) ? '会话已置顶' : '已取消置顶')" @toggle-mute="messageTips.success(toggleMuted(activeConversation.id) ? '已开启消息免打扰' : '已开启消息通知')" @clear="clearConfirmOpen = true" />
    </section>

    <nav v-if="!mobileChatOpen" class="mobile-bottom-nav" aria-label="移动端导航">
      <button :class="{ active: activeSection === 'conversations' }" type="button" @click="chooseSection('conversations')"><MessageCircle :size="21" /><span>消息</span></button>
      <button :class="{ active: activeSection === 'contacts' }" type="button" @click="chooseSection('contacts')"><Users :size="21" /><span>联系人</span></button>
      <button class="mobile-new-chat" type="button" title="新建群聊" @click="roomDialogOpen = true"><Plus :size="22" /></button>
      <button :class="{ active: activeSection === 'profile' }" type="button" @click="chooseSection('profile')"><UserRound :size="21" /><span>我的</span></button>
    </nav>

    <teleport to="body">
      <div v-if="roomDialogOpen" class="modal-layer" @click.self="roomDialogOpen = false">
        <section class="room-dialog">
          <header><div><h2>群聊</h2><p>创建新群聊或使用邀请码加入</p></div><button class="icon-button" type="button" title="关闭" @click="roomDialogOpen = false"><X :size="20" /></button></header>
          <div class="dialog-tabs"><button :class="{ active: roomMode === 'create' }" type="button" @click="roomMode = 'create'">创建群聊</button><button :class="{ active: roomMode === 'join' }" type="button" @click="roomMode = 'join'">邀请码加入</button></div>
          <form @submit.prevent="submitRoom">
            <template v-if="roomMode === 'create'">
              <label><span>群聊名称</span><input v-model.trim="roomForm.roomName" maxlength="32" placeholder="例如：IM 产品共创组" /></label>
              <label><span>群聊说明</span><textarea v-model.trim="roomForm.description" rows="3" maxlength="120" placeholder="简要说明群聊用途"></textarea></label>
            </template>
            <label v-else><span>邀请码</span><input v-model.trim="roomForm.inviteCode" placeholder="请输入群聊邀请码" /></label>
            <div class="dialog-actions"><button type="button" @click="roomDialogOpen = false">取消</button><button class="primary-button" type="submit" :disabled="roomSubmitting">{{ roomSubmitting ? "处理中..." : roomMode === "create" ? "创建" : "加入" }}</button></div>
          </form>
        </section>
      </div>
    </teleport>

    <ConfirmDialog v-model:open="clearConfirmOpen" title="清空聊天记录" description="仅清除当前浏览器中已加载的消息，不会删除服务端历史记录。" confirm-label="确认清空" @confirm="confirmClear" />
  </main>
</template>
