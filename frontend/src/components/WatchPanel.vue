<template>
  <aside class="watch-pane">
    <section class="watch-card video-card">
      <div class="pane-head compact">
        <div>
          <p class="eyebrow">Watch Room</p>
          <h2>{{ activeRoomName || "进入房间后一起看" }}</h2>
        </div>
        <div class="watch-head-actions">
          <span class="session-pill">{{ watchStatusLabel }}</span>
          <button class="icon-btn" :disabled="!activeRoomId" title="同步房间播放状态" @click="$emit('watch-control', 'get_state')">
            <RotateCw :size="18" />
          </button>
        </div>
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
      <div class="session-meta">
        <span>当前控制者：{{ watchOwnerLabel }}</span>
        <span v-if="watchSession.active">共享中 · {{ watchSession.action || "sync" }}</span>
        <span v-else>尚未发起共享</span>
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
      <button class="primary wide" :disabled="!canStartWatchSession" @click="$emit('load-video')">
        <Film :size="16" />
        {{ watchActionLabel }}
      </button>
      <button v-if="canStopWatchSession" class="ghost wide" @click="$emit('stop-watch')">结束共享</button>
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
          :disabled="!canControlVideo && watchSession.active"
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
  watchSession: { type: Object, default: () => ({ active: false, ownerId: "" }) },
  watchActionLabel: { type: String, default: "发起一起看" },
  watchStatusLabel: { type: String, default: "未共享" },
  watchOwnerLabel: { type: String, default: "暂无" },
  canStartWatchSession: { type: Boolean, default: false },
  canStopWatchSession: { type: Boolean, default: false },
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
  "stop-watch",
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

function getChunkUploadStatus() {
  if (!props.chunkUpload) return "";
  return props.chunkUpload.status?.value || props.chunkUpload.status || "";
}

const uploadStatusText = computed(() => {
  const status = getChunkUploadStatus();
  if (status === "hashing") return "计算文件指纹";
  if (status === "initializing") return "初始化上传";
  if (status === "uploading") return "分片上传中";
  if (status === "completing") return "合并文件";
  if (status === "completed") return "上传完成";
  return "准备上传";
});

const uploadProgress = computed(() => props.chunkUpload?.progress?.value || props.chunkUpload?.progress || 0);

const showUploadProgress = computed(() => {
  const status = getChunkUploadStatus();
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

<style scoped>
.watch-pane {
  display: grid;
  gap: 16px;
  padding: 18px;
  overflow: auto;
  background: transparent;
}

.video-card,
.watch-card {
  border: 1px solid var(--border);
  border-radius: var(--radius-xl);
  padding: 18px;
  background: var(--surface-soft);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(20px);
}

.video-card {
  display: grid;
  gap: 14px;
}

.pane-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.watch-head-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.pane-head h2,
.watch-card h3 {
  margin: 0;
}

.pane-head.compact {
  margin-bottom: 2px;
}

.eyebrow {
  margin: 0 0 4px;
  color: var(--primary);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.video-wrap {
  position: relative;
  overflow: hidden;
  aspect-ratio: 16 / 9;
  border-radius: 22px;
  background: #050507;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.04);
}

video {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: #050507;
}

.danmaku-layer {
  pointer-events: none;
  position: absolute;
  inset: 0;
  overflow: hidden;
}

.danmaku {
  position: absolute;
  left: 100%;
  min-width: max-content;
  border-radius: 999px;
  padding: 5px 12px;
  background: rgba(15, 12, 26, 0.56);
  color: #fff;
  font-size: 13px;
  text-shadow: 0 1px 6px rgba(0, 0, 0, 0.7);
  animation-name: danmakuMove;
  animation-timing-function: linear;
  animation-iteration-count: infinite;
}

@keyframes danmakuMove {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-760px);
  }
}

.watch-controls,
.split {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.watch-controls {
  margin-top: 2px;
}

.split {
  grid-template-columns: minmax(0, 1fr) 88px;
}

.watch-card {
  display: grid;
  gap: 12px;
}

.session-pill {
  display: inline-flex;
  align-items: center;
  min-height: 30px;
  border-radius: 999px;
  padding: 0 12px;
  background: var(--primary-soft);
  color: var(--text);
  font-size: 12px;
  white-space: nowrap;
}

.session-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 14px;
  color: var(--muted);
  font-size: 12px;
}

.room-state {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  color: var(--muted);
}

.room-state strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-box {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 48px;
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius-md);
  padding: 0 12px;
  color: var(--text);
  background: var(--surface);
  cursor: pointer;
}

.file-box input {
  display: none;
}

.upload-progress {
  display: grid;
  gap: 8px;
  color: var(--muted);
  font-size: 12px;
}

.upload-progress > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.upload-progress strong {
  color: var(--text);
}

.upload-progress progress {
  width: 100%;
  height: 6px;
  overflow: hidden;
  border: 0;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
}

.upload-progress progress::-webkit-progress-bar {
  background: rgba(255, 255, 255, 0.08);
}

.upload-progress progress::-webkit-progress-value {
  background: linear-gradient(90deg, var(--primary), var(--primary-strong));
}

.upload-progress progress::-moz-progress-bar {
  background: linear-gradient(90deg, var(--primary), var(--primary-strong));
}

.history-card {
  display: grid;
  gap: 12px;
}

.history-list {
  display: grid;
  gap: 10px;
  max-height: 260px;
  overflow: auto;
  padding-right: 2px;
}

.history-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 62px;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  padding: 12px;
  background: var(--surface);
  color: var(--text);
  text-align: left;
}

.history-item:hover,
.history-item.active {
  border-color: var(--border-strong);
  background: var(--surface-strong);
  box-shadow: var(--shadow-md);
}

.history-item .item-main {
  min-width: 0;
  flex: 1;
}

.history-item .item-main p {
  margin-top: 4px;
  color: var(--muted);
  font-size: 12px;
}

.code-box {
  border-radius: var(--radius-md);
  padding: 10px 12px;
  background: var(--primary-soft);
  color: var(--text);
  font-weight: 700;
  text-align: center;
}

@media (max-width: 1180px) {
  .watch-pane {
    padding: 16px;
  }
}
</style>
