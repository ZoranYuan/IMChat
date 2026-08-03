<script setup>
import { MessageCircle, Plus, UserRound, Users, X } from "@lucide/vue";
import { storeToRefs } from "pinia";
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRouter } from "vue-router";
import AiSummaryPanel from "../components/chat/AiSummaryPanel.vue";
import AppRail from "../components/chat/AppRail.vue";
import ChatPanel from "../components/chat/ChatPanel.vue";
import ContactsPanel from "../components/chat/ContactsPanel.vue";
import ConversationDetails from "../components/chat/ConversationDetails.vue";
import ConversationList from "../components/chat/ConversationList.vue";
import ProfilePanel from "../components/chat/ProfilePanel.vue";
import ConfirmDialog from "../components/common/ConfirmDialog.vue";
import { useResponsive } from "../composables/useResponsive.js";
import { MessageType } from "../constants/message.js";
import { useChatStore } from "../stores/chat.js";
import { messageTips } from "../utils/messageTips.js";

const router = useRouter();
const { isMobile } = useResponsive();
const chatStore = useChatStore();
const { activeConversation, activeMessages } = storeToRefs(chatStore);
const {
  state,
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
} = chatStore;

const activeSection = ref("conversations");
const mobileChatOpen = ref(false);
const detailsOpen = ref(false);
const detailsMode = ref("info");
const roomDialogOpen = ref(false);
const roomMode = ref("create");
const roomSubmitting = ref(false);
const clearConfirmOpen = ref(false);
const loadError = ref("");
const roomForm = reactive({ roomName: "", description: "", avatar: "", inviteCode: "" });

const SIDEBAR_MIN_WIDTH = 240;
const SIDEBAR_MAX_WIDTH = 420;
const DETAILS_MIN_WIDTH = 260;
const DETAILS_MAX_WIDTH = 720;
const readStoredNumber = (key, fallback) => {
  const value = Number(localStorage.getItem(key));
  return Number.isFinite(value) && value > 0 ? value : fallback;
};
const clamp = (value, min, max) => Math.min(max, Math.max(min, value));
const sidebarWidth = ref(clamp(readStoredNumber("im_sidebar_width", 300), SIDEBAR_MIN_WIDTH, SIDEBAR_MAX_WIDTH));
const detailsWidth = ref(clamp(readStoredNumber("im_details_width", 300), DETAILS_MIN_WIDTH, DETAILS_MAX_WIDTH));
const sidebarCollapsed = ref(localStorage.getItem("im_sidebar_collapsed") === "1");
const resizeTarget = ref("");
let stopResize = null;

const workspaceStyle = computed(() => ({
  "--sidebar-width": !isMobile.value && !sidebarCollapsed.value ? `${sidebarWidth.value}px` : "0px",
  "--details-width": !isMobile.value && detailsOpen.value ? `${detailsWidth.value}px` : "0px",
}));

const finishResize = () => {
  if (stopResize) stopResize();
  stopResize = null;
  resizeTarget.value = "";
  document.body.classList.remove("column-resize-active");
  localStorage.setItem("im_sidebar_width", String(sidebarWidth.value));
  localStorage.setItem("im_details_width", String(detailsWidth.value));
};

const startResize = (target, event) => {
  if (isMobile.value) return;
  event.preventDefault();
  if (target === "sidebar" && sidebarCollapsed.value) sidebarCollapsed.value = false;
  resizeTarget.value = target;
  document.body.classList.add("column-resize-active");
  const startX = event.clientX;
  const startSidebar = target === "sidebar" && sidebarCollapsed.value ? SIDEBAR_MIN_WIDTH : sidebarWidth.value;
  const startDetails = detailsWidth.value;
  const handleMove = (moveEvent) => {
    const deltaX = moveEvent.clientX - startX;
    if (target === "sidebar") sidebarWidth.value = clamp(startSidebar + deltaX, SIDEBAR_MIN_WIDTH, SIDEBAR_MAX_WIDTH);
    if (target === "details") detailsWidth.value = clamp(startDetails - deltaX, DETAILS_MIN_WIDTH, DETAILS_MAX_WIDTH);
  };
  const handleUp = () => finishResize();
  document.addEventListener("pointermove", handleMove);
  document.addEventListener("pointerup", handleUp, { once: true });
  stopResize = () => {
    document.removeEventListener("pointermove", handleMove);
    document.removeEventListener("pointerup", handleUp);
  };
};

const toggleSidebar = () => {
  sidebarCollapsed.value = !sidebarCollapsed.value;
  localStorage.setItem("im_sidebar_collapsed", sidebarCollapsed.value ? "1" : "0");
};

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
  detailsMode.value = "info";
};

const chooseConversation = async (id) => {
  mobileChatOpen.value = true;
  try {
    await selectConversation(id);
    detailsOpen.value = false;
    detailsMode.value = "info";
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
    messageTips.success(cType === MessageType.IMAGE ? "图片已发送" : cType === MessageType.VIDEO ? "视频已发送" : "文件已发送");
  } catch (error) {
    messageTips.error({ title: "发送失败", message: error.message });
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
      if (!roomForm.roomName.trim()) throw new Error("请输入群聊名称。");
      await createChatRoom(roomForm);
      messageTips.success("群聊创建成功");
    } else {
      if (!roomForm.inviteCode.trim()) throw new Error("请输入邀请码。");
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

const openDetails = () => {
  detailsMode.value = "info";
  if (detailsWidth.value > 420) detailsWidth.value = 300;
  detailsOpen.value = true;
};

const openSummary = () => {
  if (!activeConversation.value || activeConversation.value.type !== "group") return;
  detailsMode.value = "summary";
  detailsWidth.value = clamp(Math.max(detailsWidth.value, 560), DETAILS_MIN_WIDTH, DETAILS_MAX_WIDTH);
  detailsOpen.value = true;
};

const closeDetails = () => {
  detailsOpen.value = false;
  detailsMode.value = "info";
};

watch(isMobile, (mobile) => {
  if (mobile) {
    detailsOpen.value = false;
    sidebarCollapsed.value = false;
  }
});

onMounted(initialize);
onBeforeUnmount(() => finishResize());
</script>

<template>
  <main class="im-app">
    <AppRail :active-section="activeSection" :current-user="state.currentUser" :connection="state.connection"
      @select="chooseSection" @retry="retryConnection" @logout="handleLogout"
      @unsupported="(name) => messageTips.info(`${name}功能待对应接口完善后接入`)" />

    <section
      :class="['im-workspace', { 'details-visible': detailsOpen, 'sidebar-collapsed': sidebarCollapsed, resizing: resizeTarget }]"
      :style="workspaceStyle">
      <button v-if="!isMobile" class="sidebar-toggle" type="button" :title="sidebarCollapsed ? '展开会话列表' : '收起会话列表'"
        @click="toggleSidebar">{{ sidebarCollapsed ? '›' : '‹' }}</button>
      <aside :class="['directory-shell', { 'mobile-hidden': mobileChatOpen }]">
        <ConversationList v-if="activeSection === 'conversations'" :conversations="state.conversations"
          :active-id="state.activeConversationId" :loading="state.loading" @select="chooseConversation"
          @create-room="roomDialogOpen = true" />
        <ContactsPanel v-else-if="activeSection === 'contacts'" :contacts="state.contacts"
          :requests="state.friendRequests" @open-chat="handleContact" @handle-request="handleRequest"
          @add-friend="handleAddFriend" />
        <ProfilePanel v-else :current-user="state.currentUser" @logout="handleLogout" />
        <div v-if="loadError" class="directory-error"><strong>暂时无法同步数据</strong><span>{{ loadError }}</span><button
            type="button" @click="initialize">重新加载</button></div>
      </aside>
      <div v-if="!isMobile" class="column-resizer sidebar-resizer" role="separator" aria-label="调整会话列表宽度"
        @pointerdown="startResize('sidebar', $event)" @dblclick="toggleSidebar"></div>

      <ChatPanel :class="{ 'mobile-visible': mobileChatOpen }" :conversation="activeConversation"
        :messages="activeMessages" :current-user="state.currentUser" :connection="state.connection"
        :loading="state.historyLoading" :has-more="state.historyHasMore[state.activeConversationId]" :mobile="isMobile"
        :upload-status="uploadStatus" :upload-progress="uploadProgress" @back="mobileChatOpen = false"
        @details="openDetails" @open-summary="openSummary" @send="handleSend" @attachment="handleAttachment"
        @pause-upload="pauseUpload" @resume-upload="resumeUpload" @cancel-upload="cancelUpload"
        @load-older="loadOlderMessages" @unsupported="(name) => messageTips.info(`${name}功能待对应接口完善后接入`)" />

      <div v-if="detailsOpen" class="details-backdrop" @click="closeDetails"></div>
      <div v-if="detailsOpen && !isMobile" class="column-resizer details-resizer" role="separator" aria-label="调整会话详情宽度"
        @pointerdown="startResize('details', $event)"></div>
      <AiSummaryPanel v-if="detailsOpen && detailsMode === 'summary'" class="open" :conversation="activeConversation"
        @close="closeDetails" @unsupported="(name) => messageTips.info(`${name}功能待对应接口完善后接入`)" />
      <ConversationDetails v-else-if="detailsOpen" class="open" :conversation="activeConversation" @close="closeDetails"
        @toggle-pin="messageTips.success(togglePinned(activeConversation.id) ? '会话已置顶' : '已取消置顶')"
        @toggle-mute="messageTips.success(toggleMuted(activeConversation.id) ? '已开启消息免打扰' : '已开启消息通知')"
        @clear="clearConfirmOpen = true" />
    </section>

    <nav v-if="!mobileChatOpen" class="mobile-bottom-nav" aria-label="移动端导航">
      <button :class="{ active: activeSection === 'conversations' }" type="button"
        @click="chooseSection('conversations')">
        <MessageCircle :size="21" /><span>消息</span>
      </button>
      <button :class="{ active: activeSection === 'contacts' }" type="button" @click="chooseSection('contacts')">
        <Users :size="21" /><span>联系人</span>
      </button>
      <button class="mobile-new-chat" type="button" title="新建群聊" @click="roomDialogOpen = true">
        <Plus :size="22" />
      </button>
      <button :class="{ active: activeSection === 'profile' }" type="button" @click="chooseSection('profile')">
        <UserRound :size="21" /><span>我的</span>
      </button>
    </nav>

    <teleport to="body">
      <div v-if="roomDialogOpen" class="modal-layer" @click.self="roomDialogOpen = false">
        <section class="room-dialog">
          <header>
            <div>
              <h2>群聊</h2>
              <p>创建新群聊或使用邀请码加入</p>
            </div><button class="icon-button" type="button" title="关闭" @click="roomDialogOpen = false">
              <X :size="20" />
            </button>
          </header>
          <div class="dialog-tabs"><button :class="{ active: roomMode === 'create' }" type="button"
              @click="roomMode = 'create'">创建群聊</button><button :class="{ active: roomMode === 'join' }" type="button"
              @click="roomMode = 'join'">邀请码加入</button></div>
          <form @submit.prevent="submitRoom">
            <template v-if="roomMode === 'create'">
              <label><span>群聊名称</span><input v-model.trim="roomForm.roomName" maxlength="32"
                  placeholder="例如：IM 产品共创组" /></label>
              <label><span>群聊说明</span><textarea v-model.trim="roomForm.description" rows="3" maxlength="120"
                  placeholder="简要说明群聊用途"></textarea></label>
            </template>
            <label v-else><span>邀请码</span><input v-model.trim="roomForm.inviteCode" placeholder="请输入群聊邀请码" /></label>
            <div class="dialog-actions"><button type="button" @click="roomDialogOpen = false">取消</button><button
                class="primary-button" type="submit" :disabled="roomSubmitting">{{ roomSubmitting ? "处理中..." : roomMode
                  === "create"
                  ? "创建" : "加入" }}</button></div>
          </form>
        </section>
      </div>
    </teleport>

    <ConfirmDialog v-model:open="clearConfirmOpen" title="清空聊天记录" description="仅清除当前浏览器中已加载的消息，不会删除服务端历史记录。"
      confirm-label="确认清空" @confirm="confirmClear" />
  </main>
</template>
