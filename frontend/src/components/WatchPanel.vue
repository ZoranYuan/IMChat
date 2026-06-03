<template>
  <aside class="watch-pane">
    <section class="watch-card watch-stage">
      <div class="pane-head compact">
        <div>
          <p class="eyebrow">Watch Room</p>
          <h2>{{ activeRoomName || "进入房间后一起看" }}</h2>
        </div>
        <div class="watch-head-actions">
          <span class="session-pill">{{ watchStatusLabel }}</span>
          <button
            class="icon-btn"
            :disabled="!activeRoomId"
            title="同步房间播放状态"
            @click="$emit('watch-control', 'get_state')"
          >
            <RotateCw :size="18" />
          </button>
        </div>
      </div>

      <div v-if="watchSession.active && video.url" class="watch-playing">
        <div class="watch-sharing-line">
          共享人：<strong>{{ watchOwnerLabel }}</strong>
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
              v-for="(item, index) in visibleDanmaku"
              :key="item.messageId || item.seq || index"
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

        <button v-if="canStopWatchSession" class="ghost wide watch-stop" @click="$emit('stop-watch')">结束共享</button>
      </div>

      <div v-else class="watch-empty">
        <div class="watch-empty-card">
          <div class="watch-empty-copy">
            <p class="watch-empty-title">{{ activeRoomId ? "当前还没有共享视频" : "先进入房间，再开启一起看" }}</p>
            <p class="watch-empty-desc">
              房间成员可以先上传视频，或直接输入 fileId 加载视频。共享状态会保存到 Redis，重新进入房间也能同步当前播放。
            </p>
          </div>

          <label class="file-box watch-dropzone">
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
            <button class="ghost" :disabled="!fileIdInput" @click="$emit('load-file')">加载</button>
          </div>

          <input :value="video.url" placeholder="视频 URL" @input="video.url = $event.target.value.trim()" />

          <p v-if="video.fileName || video.fileId" class="muted small">
            当前待共享：{{ video.fileName || video.fileId }}
          </p>

          <button class="primary wide" :disabled="!canStartWatchSession" @click="$emit('load-video')">
            <Film :size="16" />
            {{ watchActionLabel }}
          </button>
        </div>
      </div>
    </section>

    <section class="watch-card">
      <div class="pane-head compact">
        <div>
          <p class="eyebrow">Room</p>
          <h3>房间设置</h3>
        </div>
        <div class="room-state">
          <span :class="['dot', activeRoomId ? 'ok' : '']"></span>
          <strong>{{ activeRoomName || "未进入房间" }}</strong>
        </div>
      </div>

      <div class="room-grid">
        <input :value="roomForm.roomName" placeholder="创建房间名称" @input="roomForm.roomName = $event.target.value" />
        <div class="split">
          <button class="ghost" :disabled="!token || !roomForm.roomName" @click="$emit('create-room')">创建</button>
          <button class="ghost" :disabled="!activeRoomId" @click="$emit('invite')">邀请码</button>
        </div>
        <div class="split">
          <input :value="roomForm.inviteCode" placeholder="输入邀请码加入房间" @input="roomForm.inviteCode = $event.target.value.trim()" />
          <button class="ghost" :disabled="!token || !roomForm.inviteCode" @click="$emit('join-room')">加入</button>
        </div>
        <p v-if="roomForm.inviteCodeDisplay" class="code-box">{{ roomForm.inviteCodeDisplay }}</p>
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
</script>

<style scoped>
.watch-pane {
  display: grid;
  gap: 16px;
  padding: 18px;
  overflow: auto;
  background:
    radial-gradient(circle at top right, rgba(126, 87, 194, 0.18), transparent 26%),
    radial-gradient(circle at 12% 18%, rgba(119, 104, 214, 0.14), transparent 28%),
    transparent;
}

.watch-card {
  border: 1px solid var(--border);
  border-radius: 24px;
  padding: 18px;
  background: linear-gradient(180deg, var(--surface-soft), var(--surface));
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(20px);
}

.watch-stage {
  display: grid;
  gap: 16px;
}

.pane-head {
  display: flex;
  align-items: flex-start;
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

.watch-playing,
.watch-empty-card,
.room-grid {
  display: grid;
  gap: 14px;
}

.watch-sharing-line {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 13px;
}

.watch-sharing-line strong {
  color: var(--text);
}

.video-wrap {
  position: relative;
  overflow: hidden;
  aspect-ratio: 16 / 9;
  border-radius: 24px;
  background:
    radial-gradient(circle at center, rgba(255, 255, 255, 0.06), transparent 54%),
    #050507;
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

.session-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 14px;
  color: var(--muted);
  font-size: 12px;
}

.watch-empty {
  display: grid;
}

.watch-empty-card {
  border-radius: 22px;
  padding: 18px;
  background:
    linear-gradient(180deg, rgba(127, 87, 194, 0.12), rgba(127, 87, 194, 0.04)),
    var(--surface);
  border: 1px solid var(--border);
}

.watch-empty-copy {
  display: grid;
  gap: 6px;
}

.watch-empty-title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
}

.watch-empty-desc {
  margin: 0;
  color: var(--muted);
  line-height: 1.6;
}

.watch-dropzone {
  min-height: 52px;
  background: var(--surface-strong);
}

.room-grid {
  gap: 12px;
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

.code-box {
  border-radius: 16px;
  padding: 10px 12px;
  background: var(--primary-soft);
  color: var(--text);
  font-weight: 700;
  text-align: center;
}

.watch-stop {
  justify-self: end;
  max-width: 160px;
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

@media (max-width: 1180px) {
  .watch-pane {
    padding: 16px;
  }
}
</style>
