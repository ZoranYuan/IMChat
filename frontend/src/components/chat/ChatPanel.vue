<script setup>
import { ArrowLeft, Bot, ChevronRight, FileText, Image as ImageIcon, Info, LoaderCircle, MoreHorizontal, Paperclip, Pause, Phone, Play, Send, Smile, Users, Video, X } from "@lucide/vue";
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

const emit = defineEmits(["back", "details", "open-summary", "send", "attachment", "load-older", "unsupported", "pause-upload", "resume-upload", "cancel-upload"]);
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

watch(() => props.conversation?.id, scrollToBottom);
watch(() => props.messages.length, scrollToBottom);
</script>

<template>
  <section :class="[
    'min-w-0 flex-col overflow-hidden bg-[#fbfdff]',
    mobile ? (visible ? 'fixed inset-0 z-40 flex h-full w-full' : 'hidden') : 'relative flex h-full',
  ]">
    <template v-if="conversation">
      <header class="flex min-h-16 flex-none items-center gap-3 border-b border-[#edf1f7] bg-white px-[22px] max-[767px]:px-4">
        <button v-if="mobile" class="grid size-9 place-items-center rounded-lg bg-transparent text-[#667085] hover:bg-[#f3f6fb]" type="button" title="返回" @click="$emit('back')"><ArrowLeft :size="21" /></button>
        <span class="grid size-[42px] flex-none place-items-center overflow-hidden rounded-full bg-gradient-to-br from-[#54adff] to-[#2875ff] text-sm font-bold text-white shadow-[0_10px_22px_rgba(47,109,246,0.18)]" :style="{ backgroundColor: conversation.avatarColor }">
          <img v-if="conversation.avatar" class="size-full object-cover" :src="conversation.avatar" :alt="conversation.name" />
          <Users v-else-if="conversation.type === 'group'" :size="18" />
          <span v-else>{{ conversation.name.slice(0, 1) }}</span>
        </span>
        <div class="min-w-0 flex-1">
          <h2 class="truncate text-base font-extrabold text-[#111827]">{{ conversation.name }}</h2>
          <p class="mt-0.5 flex items-center gap-1.5 text-xs text-[#8a98ac]"><i v-if="conversation.type !== 'group'" :class="conversation.online ? 'bg-[#62c894]' : 'bg-[#b9c2cf]'" class="size-1.5 rounded-full"></i>{{ conversation.type === 'group' ? `${conversation.memberCount} 位成员` : conversation.online ? "在线" : "离线" }}</p>
        </div>
        <div class="flex items-center gap-1">
          <button class="hidden size-9 place-items-center rounded-lg bg-transparent text-[#667085] hover:bg-[#f3f6fb] hover:text-[#2f6df6] min-[768px]:grid" type="button" title="语音通话" @click="$emit('unsupported', '语音通话')"><Phone :size="18" /></button>
          <button class="hidden size-9 place-items-center rounded-lg bg-transparent text-[#667085] hover:bg-[#f3f6fb] hover:text-[#2f6df6] min-[768px]:grid" type="button" title="视频通话" @click="$emit('unsupported', '视频通话')"><Video :size="19" /></button>
          <button class="grid size-9 place-items-center rounded-lg bg-transparent text-[#667085] hover:bg-[#f3f6fb] hover:text-[#2f6df6]" type="button" title="会话详情" @click="$emit('details')"><Info v-if="!mobile" :size="19" /><MoreHorizontal v-else :size="20" /></button>
        </div>
      </header>

      <div ref="messageList" class="min-h-0 flex-1 overflow-y-auto bg-[#fbfdff] px-5 py-6 max-[767px]:px-3">
        <div class="mx-auto mb-7 text-center text-[13px] text-[#98a2b3]">6月3日 星期二</div>
        <button v-if="hasMore" class="mx-auto mb-5 flex min-h-8 items-center gap-1.5 rounded-full border border-[#e4e9f2] bg-white px-3 text-[11px] text-[#7d8999] shadow-sm hover:bg-[#f5f8ff]" type="button" :disabled="loading" @click="$emit('load-older')"><LoaderCircle v-if="loading" class="animate-spin" :size="15" />{{ loading ? "加载中" : "查看更早消息" }}</button>
        <div v-if="loading && !messages.length" class="flex min-h-40 flex-col items-center justify-center gap-2 text-xs text-[#8a98ac]"><LoaderCircle class="animate-spin" :size="22" /><span>正在加载消息</span></div>
        <div v-else-if="!messages.length" class="flex min-h-40 flex-col items-center justify-center gap-1 text-center text-sm text-[#8a98ac]"><span>还没有消息</span><small class="text-xs text-[#a8b1be]">发送一条消息开始聊天</small></div>

        <article v-for="(message, index) in messages" :key="message.id" :class="message.type === 'system' ? 'justify-center' : isMine(message) ? 'flex-row-reverse' : 'flex-row'" class="mx-auto mb-[18px] flex w-full max-w-[620px] items-start gap-3">
          <template v-if="message.type === 'system'"><span class="rounded-full bg-[#eef2f8] px-3 py-1 text-[11px] text-[#8a98ac]">{{ message.content }}</span></template>
          <template v-else>
            <span class="grid size-[34px] flex-none place-items-center overflow-hidden rounded-full text-xs font-bold text-white shadow-[0_6px_16px_rgba(37,52,86,0.08)]" :style="{ backgroundColor: message.avatarColor }">{{ (message.senderName || "成员").slice(0, 1) }}</span>
            <div :class="isMine(message) ? 'items-end' : 'items-start'" class="flex max-w-[min(76%,470px)] flex-col gap-[7px]">
              <span v-if="conversation.type === 'group' && !isMine(message)" class="text-xs text-[#8b98aa]">{{ message.senderName }}</span>
              <div v-if="message.type === 'file'" class="flex w-fit max-w-full items-center gap-3 rounded-xl bg-white px-3.5 py-3 shadow-[0_10px_28px_rgba(39,57,89,0.07)]"><FileText class="flex-none text-[#2f6df6]" :size="25" /><span class="grid min-w-0 gap-1"><strong class="truncate text-[13px] text-[#1f2937]">{{ message.fileName || message.content }}</strong><small class="text-xs text-[#8a98aa]">{{ fileSizeText(message.fileSize) }}</small></span></div>
              <a v-else-if="message.type === 'image' || message.type === 'sticker'" class="block max-w-full overflow-hidden rounded-xl" :href="message.mediaUrl" target="_blank" rel="noreferrer"><img class="max-h-[280px] max-w-full rounded-xl object-cover" :src="message.thumbUrl || message.mediaUrl" :alt="message.fileName || (message.type === 'sticker' ? '表情包' : '聊天图片')" /></a>
              <div v-else-if="message.type === 'video'" class="overflow-hidden rounded-xl bg-white shadow-[0_10px_28px_rgba(39,57,89,0.07)]"><video class="max-h-[280px] max-w-full" :src="message.mediaUrl" controls preload="metadata"></video><span class="block px-3 py-2 text-xs text-[#667085]">{{ message.fileName }}</span></div>
              <div v-else :class="isMine(message) ? 'rounded-[14px_6px_14px_14px] bg-[#d8eaff] text-[#274466]' : 'rounded-[6px_14px_14px_14px] bg-white text-[#293548]'" class="w-fit max-w-full px-4 py-3 text-sm leading-[1.7] shadow-[0_10px_28px_rgba(39,57,89,0.08)]">{{ message.content }}</div>
              <div v-if="index === 1" class="flex w-[min(290px,100%)] items-center gap-3 rounded-[10px] bg-white px-3.5 py-3 shadow-[0_10px_28px_rgba(39,57,89,0.07)]"><span class="grid size-8 place-items-center rounded-lg bg-[#f1f5ff] text-[#2f6df6]"><FileText :size="20" /></span><div><strong class="block text-[13px] text-[#1f2937]">IM_Project_v1.2.fig</strong><small class="text-xs text-[#8a98aa]">Figma 文件 · 24.5MB</small></div></div>
              <span class="flex items-center gap-1 text-[11px] text-[#9aa6b7]"><time>{{ message.time }}</time><small v-if="isMine(message)">{{ message.status === 'sending' ? "发送中" : message.status === 'failed' ? "发送失败" : message.status === 'read' ? "已读" : "已发送" }}</small></span>
            </div>
          </template>
        </article>
      </div>

      <div v-if="conversation.type === 'group'" class="mx-auto mb-4 w-[min(calc(100%-56px),620px)] max-[767px]:w-[calc(100%-20px)]">
        <div class="mb-2 flex items-center gap-3 text-[11px] text-[#8b90a1]"><span class="h-px flex-1 bg-gradient-to-r from-transparent to-[#d9d4ef]"></span><p class="m-0 whitespace-nowrap">以下为 AI 生成的未读消息摘要</p><span class="h-px flex-1 bg-gradient-to-l from-transparent to-[#d9d4ef]"></span></div>
        <section class="flex items-center gap-3 rounded-[10px] border border-[#cfc3ff] bg-gradient-to-b from-[#fbfaff] to-[#f6f3ff] px-[18px] py-4 shadow-[0_14px_34px_rgba(91,53,245,0.1)]"><span class="grid size-[42px] flex-none place-items-center rounded-[14px] bg-gradient-to-br from-[#8a64ff] to-[#5b35f5] text-white shadow-[0_12px_22px_rgba(91,53,245,0.22)]"><Bot :size="20" /></span><div class="min-w-0 flex-1"><strong class="block text-[15px] text-[#17122c]">AI 已为你总结 6 条未读消息</strong><p class="mt-1 text-[13px] text-[#667085]">快速了解本次讨论的重点、决策和待办</p></div><button class="inline-flex min-h-9 items-center gap-1 rounded-lg border border-[#ded8ff] bg-white px-3 text-xs font-bold text-[#5b35f5] shadow-[0_8px_18px_rgba(91,53,245,0.08)] hover:bg-[#f8f6ff]" type="button" @click="$emit('open-summary')">查看摘要 <ChevronRight :size="15" /></button></section>
      </div>

      <footer class="grid flex-none gap-2 border-t border-[#eef2f8] bg-white px-8 pb-3 pt-2.5 max-[767px]:px-3">
        <div v-if="connection !== 'connected'" class="pb-1 text-[11px] text-[#c08526]"><i class="mr-1 inline-block size-1.5 rounded-full bg-[#d29a42]"></i>{{ connection === "connecting" ? "正在连接实时消息服务..." : "实时连接已断开，暂时无法发送消息" }}</div>
        <div v-if="isUploadVisible()" class="mx-auto w-full max-w-[620px] rounded-lg bg-[#f8f7ff] px-3 py-2 text-xs text-[#6f6980]"><div class="flex items-center justify-between"><span class="flex items-center gap-1.5"><LoaderCircle v-if="uploadStatus !== UploadStatus.PAUSED" class="animate-spin" :size="14" /><Pause v-else :size="14" />{{ uploadStatus === UploadStatus.HASHING ? "正在计算文件指纹" : uploadStatus === UploadStatus.INITIALIZING ? "正在初始化上传" : uploadStatus === UploadStatus.COMPLETING ? "正在合并分片" : uploadStatus === UploadStatus.PAUSED ? "上传已暂停" : "正在上传附件" }}</span><strong>{{ uploadProgress }}%</strong></div><progress class="mt-1 h-1 w-full accent-[#5b35f5]" :value="uploadProgress" max="100"></progress><span class="mt-1 flex justify-end gap-1"><button v-if="uploadStatus === UploadStatus.UPLOADING" class="grid size-6 place-items-center rounded bg-white hover:text-[#5b35f5]" type="button" title="暂停上传" @click="$emit('pause-upload')"><Pause :size="15" /></button><button v-if="uploadStatus === UploadStatus.PAUSED" class="grid size-6 place-items-center rounded bg-white hover:text-[#5b35f5]" type="button" title="继续上传" @click="$emit('resume-upload')"><Play :size="15" /></button><button class="grid size-6 place-items-center rounded bg-white hover:text-[#dc6570]" type="button" title="取消上传" @click="$emit('cancel-upload')"><X :size="15" /></button></span></div>
        <div class="mx-auto flex w-full max-w-[620px] items-center gap-3">
            <div class="flex items-center gap-2 text-[#667085]"><button class="grid size-7 place-items-center rounded bg-transparent hover:bg-[#f3f6fb] hover:text-[#2f6df6]" type="button" title="表情" @click="$emit('unsupported', '表情')"><Smile :size="19" /></button><button class="grid size-7 place-items-center rounded bg-transparent hover:bg-[#f3f6fb] hover:text-[#2f6df6]" type="button" title="发送图片" :disabled="isUploadVisible()" @click="imageInput?.click()"><ImageIcon :size="19" /></button><button class="grid size-7 place-items-center rounded bg-transparent hover:bg-[#f3f6fb] hover:text-[#2f6df6]" type="button" title="发送视频" :disabled="isUploadVisible()" @click="videoInput?.click()"><Video :size="19" /></button><button class="grid size-7 place-items-center rounded bg-transparent hover:bg-[#f3f6fb] hover:text-[#2f6df6]" type="button" title="发送文件" :disabled="isUploadVisible()" @click="fileInput?.click()"><Paperclip :size="19" /></button></div>
            <div class="flex min-h-11 min-w-0 flex-1 items-center gap-3 rounded-xl border border-[#e8edf5] bg-[#fbfdff] px-3 focus-within:border-[#9fc1ff] focus-within:ring-4 focus-within:ring-[#eef5ff]"><textarea v-model="draft" rows="1" placeholder="输入消息，Enter 发送，Shift + Enter 换行" class="min-h-9 min-w-0 flex-1 resize-none border-0 bg-transparent py-2 text-[13px] text-[#475467] outline-0 placeholder:text-[#a1acba]" @keydown.enter="handleEnter"></textarea><button class="flex h-[42px] w-[92px] flex-none items-center justify-center gap-2 rounded-[9px] bg-gradient-to-br from-[#2f75ff] to-[#175cff] text-sm font-bold text-white shadow-[0_12px_24px_rgba(47,109,246,0.22)] disabled:cursor-not-allowed disabled:opacity-50" type="button" title="发送消息" :disabled="!draft.trim() || connection !== 'connected' || sending" @click="submit"><span>发送</span><Send :size="16" /></button></div>
        </div>
        <input ref="imageInput" type="file" accept="image/*" hidden @change="selectAttachment($event, MessageType.IMAGE)" />
        <input ref="videoInput" type="file" accept="video/*" hidden @change="selectAttachment($event, MessageType.VIDEO)" />
        <input ref="fileInput" type="file" hidden @change="selectAttachment($event, MessageType.FILE)" />
      </footer>
    </template>

    <div v-else class="flex h-full flex-col items-center justify-center gap-3 bg-[#fbfdff] text-center text-[#8a98ac]"><span class="grid size-14 place-items-center rounded-2xl bg-[#edf3ff] text-[#2f6df6]"><Send :size="28" /></span><h2 class="m-0 text-base font-bold text-[#475467]">选择一个会话</h2><p class="m-0 text-xs">从左侧会话列表中选择联系人或群聊</p></div>
  </section>
</template>
