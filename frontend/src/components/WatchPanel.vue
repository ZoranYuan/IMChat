<template>
  <aside class="watch-pane">
    <section class="watch-card video-card">
      <div class="pane-head compact">
        <div>
          <p class="eyebrow">Watch Room</p>
          <h2>{{ activeRoomName || "进入房间后一起看" }}</h2>
        </div>
        <button class="icon-btn" :disabled="!activeRoomId" title="同步房间播放状态" @click="$emit('watch-control', 'get_state')">
          <RotateCw :size="18" />
        </button>
      </div>

      <div class="video-wrap">
        <video
          ref="localVideoRef"
          :src="video.url"
          controls
          @timeupdate="$emit('video-time-update')"
          @play="emitNativeVideoControl('play')"
          @pause="emitNativeVideoControl('pause')"
          @seeked="emitNativeVideoControl('seek')"
          @ratechange="emitNativeVideoControl('ratechange')"
          @ended="emitNativeVideoControl('ended')"
        ></video>
        <div class="danmaku-layer">
          <span
            v-for="item in visibleDanmaku"
            :key="item.messageId"
            class="danmaku"
            :style="{ top: `${item.top}%`, animationDuration: `${item.duration}s` }"
          >
            {{ item.content }}
          </span>
        </div>
      </div>
      <div class="watch-controls">
        <button class="icon-btn" :disabled="!canControlVideo" title="后退 10 秒" @click="$emit('seek-by', -10000)">
          <SkipBack :size="18" />
        </button>
        <button class="icon-btn strong" :disabled="!canControlVideo" title="播放" @click="$emit('watch-control', 'play')">
          <Play :size="18" />
        </button>
        <button class="icon-btn" :disabled="!canControlVideo" title="暂停" @click="$emit('watch-control', 'pause')">
          <Pause :size="18" />
        </button>
        <button class="icon-btn" :disabled="!canControlVideo" title="前进 10 秒" @click="$emit('seek-by', 10000)">
          <SkipForward :size="18" />
        </button>
      </div>
    </section>

    <section class="watch-card">
      <h3>房间</h3>
      <div class="room-state">
        <span :class="['dot', activeRoomId ? 'ok' : '']"></span>
        <strong>{{ activeRoomName || "未进入房间" }}</strong>
      </div>
      <input :value="roomForm.roomName" placeholder="创建房间名称" @input="roomForm.roomName = $event.target.value" />
      <div class="split">
        <button class="ghost" :disabled="!token || !roomForm.roomName" @click="$emit('create-room')">创建房间</button>
        <button class="ghost" :disabled="!activeRoomId" @click="$emit('invite')">生成邀请码</button>
      </div>
      <div class="split">
        <input :value="roomForm.inviteCode" placeholder="输入邀请码加入房间" @input="roomForm.inviteCode = $event.target.value.trim()" />
        <button class="ghost" :disabled="!token || !roomForm.inviteCode" @click="$emit('join-room')">加入</button>
      </div>
      <p v-if="roomForm.inviteCodeDisplay" class="code-box">{{ roomForm.inviteCodeDisplay }}</p>
    </section>

    <section class="watch-card">
      <h3>视频</h3>
      <p class="muted small">先进入房间，再把视频同步给房间成员。</p>
      <label class="file-box">
        <Upload :size="18" />
        <span>{{ uploadName || "上传视频文件" }}</span>
        <input type="file" accept="video/*" @change="$emit('upload', $event)" />
      </label>
      <div v-if="showUploadProgress" class="upload-progress">
        <div>
          <span>{{ uploadStatusText }}</span>
          <strong>{{ uploadProgress }}%</strong>
        </div>
        <progress :value="uploadProgress" max="100"></progress>
      </div>
      <div class="split">
        <input :value="fileIdInput" placeholder="输入视频 fileId" @input="$emit('update:fileIdInput', $event.target.value.trim())" />
        <button class="ghost" :disabled="!fileIdInput" @click="$emit('load-file')">加载视频</button>
      </div>
      <input :value="video.url" placeholder="视频 URL" @input="video.url = $event.target.value.trim()" />
      <p v-if="video.fileName || video.fileId" class="muted small">当前视频：{{ video.fileName || video.fileId }}</p>
      <button class="primary wide" :disabled="!activeRoomId || !video.url" @click="$emit('load-video')">
        <Film :size="16" />
        同步给房间
      </button>
    </section>

    <section class="watch-card history-card">
      <div class="pane-head compact">
        <div>
          <p class="eyebrow">History</p>
          <h3>房间历史视频</h3>
        </div>
        <button class="icon-btn" :disabled="!activeRoomId" title="刷新历史视频" @click="$emit('refresh-history')">
          <RotateCw :size="16" />
        </button>
      </div>
      <p v-if="!activeRoomId" class="muted small">进入房间后会显示该房间的历史视频。</p>
      <div v-else class="history-list">
        <button
          v-for="item in roomVideoHistory"
          :key="item.videoId"
          :class="['history-item', { active: item.videoId === video.fileId }]"
          @click="$emit('select-history-video', item)"
        >
          <div class="avatar">{{ (item.fileName || item.videoId || 'V').slice(0, 1).toUpperCase() }}</div>
          <div class="item-main">
            <div class="item-row">
              <strong>{{ item.fileName || item.videoId }}</strong>
              <span class="badge">{{ item.messageCount || 0 }} 条</span>
            </div>
            <p>{{ formatHistoryTime(item.latestSendTime) }}</p>
          </div>
        </button>
        <p v-if="!roomVideoHistory.length" class="empty-hint">这个房间还没有视频回放记录。</p>
      </div>
    </section>
  </aside>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { Film, Pause, Play, RotateCw, SkipBack, SkipForward, Upload } from "@lucide/vue";

const props = defineProps({
  token: { type: String, default: "" },
  activeRoomId: { type: String, default: "" },
  activeRoomName: { type: String, default: "" },
  canControlVideo: { type: Boolean, default: false },
  suppressNativeControls: { type: Boolean, default: false },
  roomForm: { type: Object, required: true },
  fileIdInput: { type: String, default: "" },
  uploadName: { type: String, default: "" },
  chunkUpload: { type: Object, default: null },
  video: { type: Object, required: true },
  roomVideoHistory: { type: Array, default: () => [] },
  visibleDanmaku: { type: Array, default: () => [] },
});

const localVideoRef = ref(null);
const emit = defineEmits([
  "watch-control",
  "seek-by",
  "video-time-update",
  "create-room",
  "join-room",
  "invite",
  "upload",
  "load-file",
  "load-video",
  "select-history-video",
  "refresh-history",
  "native-video-control",
  "update:fileIdInput",
  "update:videoEl",
]);

watch(localVideoRef, (el) => emit("update:videoEl", el), { immediate: true });

onBeforeUnmount(() => emit("update:videoEl", null));

function emitNativeVideoControl(action) {
  if (!props.canControlVideo || props.suppressNativeControls) return;
  emit("native-video-control", action);
}

const uploadStatusText = computed(() => {
  if (!props.chunkUpload) return "";
  const status = props.chunkUpload.status?.value || props.chunkUpload.status;
  if (status === "hashing") return "计算文件指纹";
  if (status === "initializing") return "初始化上传";
  if (status === "uploading") return "分片上传中";
  if (status === "completing") return "合并文件";
  if (status === "completed") return "上传完成";
  return "准备上传";
});

const uploadProgress = computed(() => props.chunkUpload?.progress?.value || props.chunkUpload?.progress || 0);

const showUploadProgress = computed(() => {
  if (!props.chunkUpload) return false;
  const status = props.chunkUpload.status?.value || props.chunkUpload.status;
  return ["hashing", "initializing", "uploading", "completing", "completed"].includes(status);
});

function formatHistoryTime(ts) {
  if (!ts) return "刚刚";
  return new Date(ts).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}
</script>
