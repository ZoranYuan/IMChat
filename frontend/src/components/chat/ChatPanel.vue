<script setup>
import { ArrowLeft, FileText, Image as ImageIcon, Info, LoaderCircle, MoreHorizontal, Paperclip, Pause, Play, Send, Users, Video, X } from "@lucide/vue";
import { nextTick, ref, watch } from "vue";
import { MessageType } from "../../constants/message.js";
import { UploadStatus } from "../../constants/upload.js";

const props = defineProps({
  conversation: { type: Object, default: null },
  messages: { type: Array, default: () => [] },
  currentUser: { type: Object, required: true },
  connection: { type: String, default: "disconnected" },
  loading: { type: Boolean, default: false },
  hasMore: { type: Boolean, default: false },
  mobile: { type: Boolean, default: false },
  visible: { type: Boolean, default: true },
  uploadStatus: { type: String, default: UploadStatus.IDLE },
  uploadProgress: { type: Number, default: 0 },
});

const emit = defineEmits(["back", "details", "send", "attachment", "load-older", "pause-upload", "resume-upload", "cancel-upload"]);
const draft = ref("");
const sending = ref(false);
const messageList = ref(null);
const imageInput = ref(null);
const videoInput = ref(null);
const fileInput = ref(null);
const composerHeight = ref(160);
const isResizingComposer = ref(false);
const resizeStartY = ref(0);
const resizeStartHeight = ref(160);
const MIN_COMPOSER_HEIGHT = 160;
const MAX_COMPOSER_HEIGHT = 300;

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
const isUploadVisible = () => ![
  UploadStatus.IDLE,
  UploadStatus.COMPLETED,
  UploadStatus.FAILED,
  UploadStatus.CANCELED,
].includes(props.uploadStatus);
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

const clampComposerHeight = (height) => Math.min(MAX_COMPOSER_HEIGHT, Math.max(MIN_COMPOSER_HEIGHT, height));

const startComposerResize = (event) => {
  event.preventDefault();
  resizeStartY.value = event.clientY;
  resizeStartHeight.value = composerHeight.value;
  isResizingComposer.value = true;
  event.currentTarget.setPointerCapture?.(event.pointerId);
};

const resizeComposer = (event) => {
  if (!isResizingComposer.value) return;
  composerHeight.value = clampComposerHeight(resizeStartHeight.value - (event.clientY - resizeStartY.value));
};

const stopComposerResize = (event) => {
  if (!isResizingComposer.value) return;
  isResizingComposer.value = false;
  if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
    event.currentTarget.releasePointerCapture(event.pointerId);
  }
};

watch(() => props.conversation?.id, scrollToBottom);
watch(() => props.messages.length, scrollToBottom);
</script>

<template>
  <section :class="[
    'chat-panel min-w-0 flex-col overflow-hidden bg-[#faf9ff]',
    mobile ? (visible ? 'fixed inset-0 z-40 flex h-full w-full' : 'hidden') : 'relative flex h-full',
  ]">
    <template v-if="conversation">
      <header class="flex min-h-16 flex-none items-center gap-3 border-b border-[#edf1f7] bg-white px-[22px] max-[767px]:px-4">
        <button v-if="mobile" class="grid size-9 place-items-center rounded-lg bg-transparent text-[#667085] hover:bg-[#f3f6fb]" type="button" title="返回" @click="$emit('back')"><ArrowLeft :size="21" /></button>
        <span class="grid size-[42px] flex-none place-items-center overflow-hidden rounded-full bg-gradient-to-br from-[#9a7bff] to-[#5b35f5] text-sm font-bold text-white shadow-[0_10px_22px_rgba(91,53,245,0.2)]" :style="{ backgroundColor: conversation.avatarColor }">
          <img v-if="conversation.avatar" class="size-full object-cover" :src="conversation.avatar" :alt="conversation.name" />
          <Users v-else-if="conversation.type === 'group'" :size="18" />
          <span v-else>{{ conversation.name.slice(0, 1) }}</span>
        </span>
        <div class="min-w-0 flex-1">
          <h2 class="truncate text-[15px] font-extrabold text-[#111827]">{{ conversation.name }}</h2>
          <p class="mt-0.5 flex items-center gap-1.5 text-[11px] text-[#8a98ac]"><i v-if="conversation.type !== 'group'" :class="conversation.online ? 'bg-[#62c894]' : 'bg-[#b9c2cf]'" class="size-1.5 rounded-full"></i>{{ conversation.type === 'group' ? `${conversation.memberCount} 位成员` : conversation.online ? "在线" : "离线" }}</p>
        </div>
        <div class="flex items-center gap-1">
          <button class="grid size-9 place-items-center rounded-lg bg-transparent text-[#667085] hover:bg-[#f3efff] hover:text-[#5b35f5]" type="button" title="会话详情" @click="$emit('details')"><Info v-if="!mobile" :size="19" /><MoreHorizontal v-else :size="20" /></button>
        </div>
      </header>

      <div ref="messageList" class="flex min-h-0 flex-1 flex-col items-stretch overflow-y-auto scroll-smooth bg-[linear-gradient(180deg,#fbfaff_0%,#f7f8fc_100%)] px-[clamp(14px,2.2vw,28px)] pb-3 pt-[22px] max-[767px]:px-3">
        <button v-if="hasMore" class="mx-auto mb-5 flex min-h-8 items-center gap-1.5 rounded-full border border-[#e4e9f2] bg-white px-3 text-[11px] text-[#7d8999] shadow-sm hover:bg-[#f5f8ff]" type="button" :disabled="loading" @click="$emit('load-older')"><LoaderCircle v-if="loading" class="animate-spin" :size="15" />{{ loading ? "加载中" : "查看更早消息" }}</button>
        <div v-if="loading && !messages.length" class="flex min-h-40 flex-col items-center justify-center gap-2 text-xs text-[#8a98ac]"><LoaderCircle class="animate-spin" :size="22" /><span>正在加载消息</span></div>
        <div v-else-if="!messages.length" class="flex min-h-40 flex-col items-center justify-center gap-1 text-center text-sm text-[#8a98ac]"><span>还没有消息</span><small class="text-xs text-[#a8b1be]">发送一条消息开始聊天</small></div>

        <article v-for="(message, index) in messages" :key="message.id" :class="message.type === 'system' ? 'justify-center' : isMine(message) ? 'flex-row-reverse' : 'flex-row'" class="mx-auto mb-[18px] flex w-full max-w-[780px] items-start gap-3 min-[1500px]:max-w-[840px]">
          <template v-if="message.type === 'system'"><span class="rounded-full bg-[#eef2f8] px-3 py-1 text-[11px] text-[#8a98ac]">{{ message.content }}</span></template>
          <template v-else>
            <span class="grid size-[34px] flex-none place-items-center overflow-hidden rounded-full text-xs font-bold text-white shadow-[0_6px_16px_rgba(37,52,86,0.08)]" :style="{ backgroundColor: message.avatarColor }">{{ (message.senderName || "成员").slice(0, 1) }}</span>
            <div :class="isMine(message) ? 'items-end' : 'items-start'" class="flex max-w-[min(78%,560px)] flex-col gap-[7px] max-[767px]:max-w-[84%]">
              <span v-if="conversation.type === 'group' && !isMine(message)" class="text-xs text-[#8b98aa]">{{ message.senderName }}</span>
              <div v-if="message.type === 'file'" class="flex w-fit max-w-full items-center gap-3 rounded-xl bg-white px-3.5 py-3 shadow-[0_10px_28px_rgba(91,53,245,0.08)]"><FileText class="flex-none text-[#5b35f5]" :size="25" /><span class="grid min-w-0 gap-1"><strong class="truncate text-[13px] text-[#1f2937]">{{ message.fileName || message.content }}</strong><small class="text-xs text-[#8a98aa]">{{ fileSizeText(message.fileSize) }}</small></span></div>
              <a v-else-if="message.type === 'image' || message.type === 'sticker'" class="block max-w-full overflow-hidden rounded-xl" :href="message.mediaUrl" target="_blank" rel="noreferrer"><img class="max-h-[280px] max-w-full rounded-xl object-cover" :src="message.thumbUrl || message.mediaUrl" :alt="message.fileName || (message.type === 'sticker' ? '表情包' : '聊天图片')" /></a>
              <div v-else-if="message.type === 'video'" class="overflow-hidden rounded-xl bg-white shadow-[0_10px_28px_rgba(91,53,245,0.08)]"><video class="max-h-[280px] max-w-full" :src="message.mediaUrl" controls preload="metadata"></video><span class="block px-3 py-2 text-xs text-[#667085]">{{ message.fileName }}</span></div>
              <div v-else :class="isMine(message) ? 'rounded-[14px_6px_14px_14px] bg-[#e4dcff] text-[#3c2c72]' : 'rounded-[6px_14px_14px_14px] bg-white text-[#293548]'" class="w-fit max-w-full px-4 py-3 text-[13px] leading-[1.7] shadow-[0_10px_28px_rgba(91,53,245,0.08)]">{{ message.content }}</div>
              <span class="flex items-center gap-1 text-[11px] text-[#9aa6b7]"><time>{{ message.time }}</time><small v-if="isMine(message)">{{ message.status === 'sending' ? "发送中" : message.status === 'failed' ? "发送失败" : message.status === 'read' ? "已读" : "已发送" }}</small></span>
            </div>
          </template>
        </article>
      </div>

      <footer class="flex flex-none flex-col gap-2 border-t border-[#ebe7f5] bg-[#fbfaff] px-8 pb-4 pt-2.5 max-[767px]:px-3 max-[767px]:pb-3">
        <div v-if="connection !== 'connected'" class="mx-auto w-full max-w-[1040px] text-[11px] text-[#c08526]"><i class="mr-1 inline-block size-1.5 rounded-full bg-[#d29a42]"></i>{{ connection === "connecting" ? "正在连接实时消息服务..." : "实时连接已断开，暂时无法发送消息" }}</div>
        <div v-if="isUploadVisible()" class="mx-auto w-full max-w-[1040px] rounded-xl border border-[#e9e3fb] bg-white px-3 py-2 text-xs text-[#6f6980] shadow-[0_5px_18px_rgba(91,53,245,0.04)]"><div class="flex items-center justify-between"><span class="flex items-center gap-1.5"><LoaderCircle v-if="uploadStatus !== UploadStatus.PAUSED" class="animate-spin" :size="14" /><Pause v-else :size="14" />{{ uploadStatus === UploadStatus.HASHING ? "正在计算文件指纹" : uploadStatus === UploadStatus.INITIALIZING ? "正在初始化上传" : uploadStatus === UploadStatus.COMPLETING ? "正在合并分片" : uploadStatus === UploadStatus.PAUSED ? "上传已暂停" : "正在上传附件" }}</span><strong>{{ uploadProgress }}%</strong></div><progress class="mt-1 h-1 w-full accent-[#5b35f5]" :value="uploadProgress" max="100"></progress><span class="mt-1 flex justify-end gap-1"><button v-if="uploadStatus === UploadStatus.UPLOADING" class="grid size-6 place-items-center rounded bg-[#faf9ff] hover:text-[#5b35f5]" type="button" title="暂停上传" @click="$emit('pause-upload')"><Pause :size="15" /></button><button v-if="uploadStatus === UploadStatus.PAUSED" class="grid size-6 place-items-center rounded bg-[#faf9ff] hover:text-[#5b35f5]" type="button" title="继续上传" @click="$emit('resume-upload')"><Play :size="15" /></button><button class="grid size-6 place-items-center rounded bg-[#faf9ff] hover:text-[#dc6570]" type="button" title="取消上传" @click="$emit('cancel-upload')"><X :size="15" /></button></span></div>
        <div class="mx-auto w-full max-w-[1040px]">
          <div :class="isResizingComposer ? 'select-none' : ''" :style="{ height: `${composerHeight}px` }" class="relative flex w-full min-h-0 transition-none">
            <button class="group absolute inset-x-0 top-0 z-10 h-2 cursor-ns-resize touch-none bg-transparent" type="button" aria-label="调整输入框高度"
              @pointerdown="startComposerResize" @pointermove="resizeComposer" @pointerup="stopComposerResize" @pointercancel="stopComposerResize">
              <span class="mx-auto block h-1 w-10 rounded-full bg-[#e5defa] transition-colors group-hover:bg-[#cfc5ff] group-active:bg-[#a493ff]"></span>
            </button>
            <div class="flex h-full min-h-0 min-w-0 flex-col overflow-hidden rounded-2xl border border-[#e6e0f4] bg-white shadow-[0_8px_26px_rgba(91,53,245,0.06)]">
              <div class="flex h-11 flex-none items-center gap-1 border-b border-[#f0ecf8] px-3 pt-1 text-[#77718c]">
                <button class="grid size-7 place-items-center rounded-lg bg-transparent transition-colors hover:bg-[#f3efff] hover:text-[#5b35f5] disabled:cursor-not-allowed disabled:opacity-40" type="button" title="发送图片" :disabled="isUploadVisible()" @click="imageInput?.click()"><ImageIcon :size="18" /></button>
                <button class="grid size-7 place-items-center rounded-lg bg-transparent transition-colors hover:bg-[#f3efff] hover:text-[#5b35f5] disabled:cursor-not-allowed disabled:opacity-40" type="button" title="发送视频" :disabled="isUploadVisible()" @click="videoInput?.click()"><Video :size="18" /></button>
                <button class="grid size-7 place-items-center rounded-lg bg-transparent transition-colors hover:bg-[#f3efff] hover:text-[#5b35f5] disabled:cursor-not-allowed disabled:opacity-40" type="button" title="发送文件" :disabled="isUploadVisible()" @click="fileInput?.click()"><Paperclip :size="18" /></button>
                <span class="ml-auto hidden text-[10px] text-[#aaa6b5] min-[768px]:inline">Enter 发送 · Shift + Enter 换行</span>
              </div>
              <textarea v-model="draft" rows="1" placeholder="输入消息..." class="min-h-0 min-w-0 flex-1 resize-none overflow-y-auto border-0 bg-transparent px-4 py-3 text-[13px] leading-6 text-[#475467] outline-0 placeholder:text-[#b0aabd]" @keydown.enter="handleEnter"></textarea>
              <div class="flex h-14 flex-none items-center justify-end border-t border-[#f4f1f9] px-3">
                <button class="flex h-10 min-w-[92px] items-center justify-center gap-2 rounded-[10px] px-4 bg-gradient-to-br from-[#7355ff] to-[#4b2bd7] text-[13px] font-bold text-white shadow-[0_10px_20px_rgba(91,53,245,0.2)] transition-[box-shadow,transform] duration-150 hover:-translate-y-0.5 hover:shadow-[0_14px_26px_rgba(91,53,245,0.28)] disabled:cursor-not-allowed disabled:opacity-50" type="button" title="发送消息" :disabled="!draft.trim() || connection !== 'connected' || sending" @click="submit"><span>发送</span><Send :size="16" /></button>
              </div>
            </div>
          </div>
        </div>
        <input ref="imageInput" type="file" accept="image/*" hidden @change="selectAttachment($event, MessageType.IMAGE)" />
        <input ref="videoInput" type="file" accept="video/*" hidden @change="selectAttachment($event, MessageType.VIDEO)" />
        <input ref="fileInput" type="file" hidden @change="selectAttachment($event, MessageType.FILE)" />
      </footer>
    </template>

    <div v-else class="flex h-full flex-col items-center justify-center gap-3 bg-[#faf9ff] text-center text-[#8a98ac]"><span class="grid size-14 place-items-center rounded-2xl bg-[#eee9ff] text-[#5b35f5]"><Send :size="28" /></span><h2 class="m-0 text-base font-bold text-[#475467]">选择一个会话</h2><p class="m-0 text-xs">从左侧会话列表中选择联系人或群聊</p></div>
  </section>
</template>
