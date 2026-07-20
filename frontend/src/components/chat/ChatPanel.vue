<script setup>
import { ArrowLeft, FileText, Image as ImageIcon, Info, LoaderCircle, MoreHorizontal, Paperclip, Pause, Phone, Play, Send, Smile, Users, Video, X } from "@lucide/vue";
import { nextTick, ref, watch } from "vue";

const props = defineProps({
  conversation: { type: Object, default: null },
  messages: { type: Array, default: () => [] },
  currentUser: { type: Object, required: true },
  connection: { type: String, default: "disconnected" },
  loading: { type: Boolean, default: false },
  hasMore: { type: Boolean, default: false },
  mobile: { type: Boolean, default: false },
  uploadStatus: { type: String, default: "idle" },
  uploadProgress: { type: Number, default: 0 },
});

const emit = defineEmits(["back", "details", "send", "attachment", "load-older", "unsupported", "pause-upload", "resume-upload", "cancel-upload"]);
const draft = ref("");
const sending = ref(false);
const messageList = ref(null);
const imageInput = ref(null);
const videoInput = ref(null);
const fileInput = ref(null);

const scrollToBottom = () => nextTick(() => {
  if (messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight;
});

const submit = async () => {
  const text = draft.value.trim();
  if (!text || sending.value) return;
  sending.value = true;
  try {
    emit("send", text);
    draft.value = "";
    scrollToBottom();
  } finally {
    sending.value = false;
  }
};

const handleEnter = (event) => {
  if (event.isComposing || event.shiftKey) return;
  event.preventDefault();
  submit();
};

const isMine = (message) => message.senderId === (props.currentUser.userId || props.currentUser.id);
const isUploadVisible = () => !["idle", "completed", "failed", "canceled"].includes(props.uploadStatus);
const fileSizeText = (size) => {
  const bytes = Number(size) || 0;
  if (!bytes) return "";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
};
const selectAttachment = (event, cType) => {
  const file = event.target.files?.[0];
  if (file) emit("attachment", file, cType);
  event.target.value = "";
};

watch(() => props.conversation?.id, scrollToBottom);
watch(() => props.messages.length, scrollToBottom);
</script>

<template>
  <section :class="['chat-panel', { mobile }]">
    <template v-if="conversation">
      <header class="chat-header">
        <button v-if="mobile" class="icon-button" type="button" title="返回" @click="$emit('back')"><ArrowLeft :size="21" /></button>
        <span class="chat-header-avatar" :style="{ backgroundColor: conversation.avatarColor }">
          <img v-if="conversation.avatar" :src="conversation.avatar" :alt="conversation.name" />
          <Users v-else-if="conversation.type === 'group'" :size="18" />
          <span v-else>{{ conversation.name.slice(0, 1) }}</span>
        </span>
        <div class="chat-title">
          <h2>{{ conversation.name }}</h2>
          <p v-if="conversation.type === 'group'">{{ conversation.memberCount }} 位成员</p>
          <p v-else><i :class="{ online: conversation.online }"></i>{{ conversation.online ? "在线" : "离线" }}</p>
        </div>
        <div class="chat-header-actions">
          <button class="icon-button desktop-action" type="button" title="语音通话" @click="$emit('unsupported', '语音通话')"><Phone :size="18" /></button>
          <button class="icon-button desktop-action" type="button" title="视频通话" @click="$emit('unsupported', '视频通话')"><Video :size="19" /></button>
          <button class="icon-button" type="button" title="会话详情" @click="$emit('details')"><Info v-if="!mobile" :size="19" /><MoreHorizontal v-else :size="20" /></button>
        </div>
      </header>

      <div ref="messageList" class="message-list">
        <div class="conversation-notice">消息仅对会话成员可见</div>
        <button v-if="hasMore" class="load-history" type="button" :disabled="loading" @click="$emit('load-older')">
          <LoaderCircle v-if="loading" class="spin" :size="15" />{{ loading ? "加载中" : "查看更早消息" }}
        </button>
        <div v-if="loading && !messages.length" class="messages-loading"><LoaderCircle class="spin" :size="22" /><span>正在加载消息</span></div>
        <div v-else-if="!messages.length" class="messages-empty"><span>还没有消息</span><small>发送一条消息开始聊天</small></div>

        <article v-for="message in messages" :key="message.id" :class="['message-row', { mine: isMine(message), system: message.type === 'system' }]">
          <template v-if="message.type === 'system'"><span>{{ message.content }}</span></template>
          <template v-else>
            <span class="message-avatar" :style="{ backgroundColor: message.avatarColor }">{{ (message.senderName || "成员").slice(0, 1) }}</span>
            <div class="message-body">
              <span v-if="conversation.type === 'group' && !isMine(message)" class="sender-name">{{ message.senderName }}</span>
              <div v-if="message.type === 'file'" class="file-message">
                <FileText :size="25" /><span><strong>{{ message.fileName || message.content }}</strong><small>{{ fileSizeText(message.fileSize) }}</small></span>
              </div>
              <a v-else-if="message.type === 'image'" class="image-message" :href="message.mediaUrl" target="_blank" rel="noreferrer"><img :src="message.thumbUrl || message.mediaUrl" :alt="message.fileName || '聊天图片'" /></a>
              <div v-else-if="message.type === 'video'" class="video-message"><video :src="message.mediaUrl" controls preload="metadata"></video><span>{{ message.fileName }}</span></div>
              <div v-else class="message-bubble">{{ message.content }}</div>
              <span class="message-meta"><time>{{ message.time }}</time><small v-if="isMine(message)">{{ message.status === 'sending' ? "发送中" : message.status === 'failed' ? "发送失败" : message.status === 'read' ? "已读" : "已发送" }}</small></span>
            </div>
          </template>
        </article>
      </div>

      <footer class="composer-area">
        <div v-if="connection !== 'connected'" class="connection-banner">
          <i :class="connection"></i>{{ connection === "connecting" ? "正在连接实时消息服务..." : "实时连接已断开，暂时无法发送消息" }}
        </div>
        <div v-if="isUploadVisible()" class="upload-progress-row">
          <div><span><LoaderCircle v-if="uploadStatus !== 'paused'" class="spin" :size="14" /><Pause v-else :size="14" />{{ uploadStatus === "hashing" ? "正在计算文件指纹" : uploadStatus === "initializing" ? "正在初始化上传" : uploadStatus === "completing" ? "正在合并分片" : uploadStatus === "paused" ? "上传已暂停" : "正在上传附件" }}</span><strong>{{ uploadProgress }}%</strong></div>
          <progress :value="uploadProgress" max="100"></progress>
          <span class="upload-actions"><button v-if="uploadStatus === 'uploading'" type="button" title="暂停上传" @click="$emit('pause-upload')"><Pause :size="15" /></button><button v-if="uploadStatus === 'paused'" type="button" title="继续上传" @click="$emit('resume-upload')"><Play :size="15" /></button><button type="button" title="取消上传" @click="$emit('cancel-upload')"><X :size="15" /></button></span>
        </div>
        <div class="composer-toolbar">
          <button type="button" title="表情" @click="$emit('unsupported', '表情')"><Smile :size="19" /></button>
          <button type="button" title="发送图片" :disabled="isUploadVisible()" @click="imageInput?.click()"><ImageIcon :size="19" /></button>
          <button type="button" title="发送视频" :disabled="isUploadVisible()" @click="videoInput?.click()"><Video :size="19" /></button>
          <button type="button" title="发送文件" :disabled="isUploadVisible()" @click="fileInput?.click()"><Paperclip :size="19" /></button>
        </div>
        <div class="composer-main">
          <textarea v-model="draft" rows="1" :disabled="connection !== 'connected'" placeholder="输入消息，Enter 发送，Shift + Enter 换行" @keydown.enter="handleEnter"></textarea>
          <button class="send-button" type="button" title="发送消息" :disabled="!draft.trim() || connection !== 'connected' || sending" @click="submit"><Send :size="18" /></button>
        </div>
        <input ref="imageInput" type="file" accept="image/*" hidden @change="selectAttachment($event, 2)" />
        <input ref="videoInput" type="file" accept="video/*" hidden @change="selectAttachment($event, 3)" />
        <input ref="fileInput" type="file" hidden @change="selectAttachment($event, 5)" />
      </footer>
    </template>

    <div v-else class="no-conversation">
      <span><Send :size="28" /></span><h2>选择一个会话</h2><p>从左侧会话列表中选择联系人或群聊</p>
    </div>
  </section>
</template>
