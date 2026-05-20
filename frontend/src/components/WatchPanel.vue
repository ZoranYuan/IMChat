<template>
  <aside class="watch-pane">
    <section class="watch-card video-card">
      <div class="pane-head compact">
        <div>
          <p class="eyebrow">Watch Together</p>
          <h2>一起看</h2>
        </div>
        <button class="icon-btn" :disabled="!activeRoomId" title="同步状态" @click="$emit('watch-control', 'get_state')">
          <RotateCw :size="18" />
        </button>
      </div>

      <div class="video-wrap">
        <video :ref="setVideoRef" :src="video.url" controls @timeupdate="$emit('video-time-update')"></video>
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
        <button class="icon-btn" :disabled="!activeRoomId" title="后退 10 秒" @click="$emit('seek-by', -10000)">
          <SkipBack :size="18" />
        </button>
        <button class="icon-btn strong" :disabled="!activeRoomId" title="播放" @click="$emit('watch-control', 'play')">
          <Play :size="18" />
        </button>
        <button class="icon-btn" :disabled="!activeRoomId" title="暂停" @click="$emit('watch-control', 'pause')">
          <Pause :size="18" />
        </button>
        <button class="icon-btn" :disabled="!activeRoomId" title="前进 10 秒" @click="$emit('seek-by', 10000)">
          <SkipForward :size="18" />
        </button>
      </div>
    </section>

    <section class="watch-card">
      <h3>房间</h3>
      <input :value="roomForm.roomName" placeholder="房间名称" @input="roomForm.roomName = $event.target.value" />
      <div class="split">
        <button class="ghost" :disabled="!token" @click="$emit('create-room')">创建</button>
        <button class="ghost" :disabled="!activeRoomId" @click="$emit('invite')">邀请码</button>
      </div>
      <div class="split">
        <input :value="roomForm.inviteCode" placeholder="输入邀请码" @input="roomForm.inviteCode = $event.target.value.trim()" />
        <button class="ghost" :disabled="!token" @click="$emit('join-room')">加入</button>
      </div>
      <p class="muted small">当前房间：{{ activeRoomId || "未选择" }}</p>
      <p v-if="roomForm.inviteCodeDisplay" class="code-box">{{ roomForm.inviteCodeDisplay }}</p>
    </section>

    <section class="watch-card">
      <h3>视频</h3>
      <label class="file-box">
        <Upload :size="18" />
        <span>{{ uploadName || "上传视频文件" }}</span>
        <input type="file" accept="video/*" @change="$emit('upload', $event)" />
      </label>
      <div class="split">
        <input :value="fileIdInput" placeholder="fileId" @input="$emit('update:fileIdInput', $event.target.value.trim())" />
        <button class="ghost" :disabled="!fileIdInput" @click="$emit('load-file')">加载</button>
      </div>
      <input :value="video.url" placeholder="视频 URL" @input="video.url = $event.target.value.trim()" />
      <button class="primary wide" :disabled="!activeRoomId || !video.url" @click="$emit('load-video')">
        <Film :size="16" />
        同步到房间
      </button>
    </section>
  </aside>
</template>

<script setup>
import { Film, Pause, Play, RotateCw, SkipBack, SkipForward, Upload } from "@lucide/vue";

const props = defineProps({
  token: { type: String, default: "" },
  activeRoomId: { type: String, default: "" },
  roomForm: { type: Object, required: true },
  fileIdInput: { type: String, default: "" },
  uploadName: { type: String, default: "" },
  video: { type: Object, required: true },
  videoRef: { type: Object, required: true },
  visibleDanmaku: { type: Array, default: () => [] },
});

defineEmits([
  "watch-control",
  "seek-by",
  "video-time-update",
  "create-room",
  "join-room",
  "invite",
  "upload",
  "load-file",
  "load-video",
  "update:fileIdInput",
]);

function setVideoRef(el) {
  props.videoRef.value = el;
}
</script>
