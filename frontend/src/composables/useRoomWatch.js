import { computed, nextTick, reactive, ref } from "vue";
import { createRoom, getDanmaku, getFile, getInviteCode, joinRoom } from "../api";
import { useChunkUpload } from "./useChunkUpload";

export function useRoomWatch({ token, currentUser, showMessage, sendFrame, openConversation, wsConnected }) {
  const roomForm = reactive({ roomName: "一起看房间", inviteCode: "", inviteCodeDisplay: "" });
  const activeRoomId = ref("");
  const fileIdInput = ref("");
  const uploadName = ref("");
  const video = reactive({ fileId: "", url: "", objectKey: "", fileName: "" });
  const watchSession = reactive({
    active: false,
    roomId: "",
    ownerId: "",
    action: "",
    videoId: "",
    videoUrl: "",
    updatedAtMs: 0,
    clientTimeMs: 0,
    durationMs: 0,
    playbackRate: 1,
    isPlaying: false,
  });
  const videoRef = ref(null);
  const danmakuItems = ref([]);
  const currentVideoTime = ref(0);
  const chunkUpload = useChunkUpload();
  const applyingWatchState = ref(false);

  const hasActiveWatchOwner = computed(
    () => Boolean(watchSession.active && watchSession.ownerId && watchSession.ownerId !== currentUser.userId),
  );

  const canControlWatchVideo = computed(() => Boolean(activeRoomId.value && video.url && !hasActiveWatchOwner.value));
  const canStartWatchSession = computed(() => Boolean(activeRoomId.value && video.url && !hasActiveWatchOwner.value));
  const canStopWatchSession = computed(
    () => Boolean(activeRoomId.value && watchSession.active && watchSession.ownerId === currentUser.userId),
  );

  const watchOwnerLabel = computed(() => {
    if (!watchSession.active) return "暂无";
    if (!watchSession.ownerId || watchSession.ownerId === currentUser.userId) return "你";
    return `成员 ${watchSession.ownerId.slice(0, 6)}`;
  });

  const watchStatusLabel = computed(() => {
    if (!activeRoomId.value) return "未进入房间";
    if (!watchSession.active) return "未共享";
    if (watchSession.ownerId === currentUser.userId) return "由你共享";
    return `由 ${watchOwnerLabel.value} 共享`;
  });

  const watchActionLabel = computed(() => {
    if (hasActiveWatchOwner.value) return "正在共享中";
    if (watchSession.active && watchSession.ownerId === currentUser.userId) return "更新共享";
    return "发起一起看";
  });

  const visibleDanmaku = computed(() => {
    const nowMs = currentVideoTime.value * 1000;
    return danmakuItems.value
      .filter((item) => Math.abs(item.timeMs - nowMs) < 4500)
      .slice(-8)
      .map((item, index) => ({
        ...item,
        top: 12 + (index % 6) * 12,
        duration: 9 + (index % 3),
      }));
  });

  function selectRoomConversation(conversation) {
    if (!conversation?.conversationId) return;
    activeRoomId.value = conversation.conversationId;
    danmakuItems.value = [];
    resetWatchSession();
  }

  function appendDanmakuItem(item) {
    if (!item) return;
    const exists = danmakuItems.value.some((current) => {
      if (item.messageId && current.messageId && item.messageId === current.messageId) return true;
      if (item.clientMsgId && current.clientMsgId && item.clientMsgId === current.clientMsgId) return true;
      return false;
    });
    if (exists) return;
    danmakuItems.value.push({ ...item });
  }

  function mergeDanmakuItems(existing = [], incoming = []) {
    const merged = [];
    const seen = new Set();
    for (const item of [...existing, ...incoming]) {
      if (!item) continue;
      const key = item.messageId || item.clientMsgId || `${item.senderId || ""}:${item.timeMs || 0}:${item.content || ""}`;
      if (seen.has(key)) continue;
      seen.add(key);
      merged.push({ ...item });
    }
    merged.sort((a, b) => (Number(a.timeMs) || 0) - (Number(b.timeMs) || 0));
    return merged;
  }

  function resetWatchSession() {
    Object.assign(watchSession, {
      active: false,
      roomId: activeRoomId.value || "",
      ownerId: "",
      action: "",
      videoId: "",
      videoUrl: "",
      updatedAtMs: 0,
      clientTimeMs: 0,
      durationMs: 0,
      playbackRate: 1,
      isPlaying: false,
    });
  }

  function clearVideoContext() {
    Object.assign(video, { fileId: "", url: "", objectKey: "", fileName: "" });
    fileIdInput.value = "";
    danmakuItems.value = [];
    currentVideoTime.value = 0;
  }

  function ensureWatchEditable() {
    if (hasActiveWatchOwner.value) {
      showMessage(`当前由 ${watchOwnerLabel.value} 共享，请先结束后再发起`);
      return false;
    }
    return true;
  }

  function sendWatchControl(action, patch = {}) {
    if (!activeRoomId.value) {
      showMessage("请先进入房间");
      return;
    }
    if (action !== "get_state" && action !== "stop" && !video.url) {
      showMessage("请先加载视频");
      return;
    }
    if (action !== "get_state" && hasActiveWatchOwner.value) {
      showMessage(`当前由 ${watchOwnerLabel.value} 共享，请先结束后再发起`);
      return;
    }
    const current = videoRef.value ? Math.floor(videoRef.value.currentTime * 1000) : 0;
    sendFrame("watch_video_control", "watchControl", {
      roomId: activeRoomId.value,
      action,
      videoId: video.fileId,
      videoUrl: video.url,
      positionMs: current,
      durationMs: videoRef.value ? Math.floor((videoRef.value.duration || 0) * 1000) : 0,
      playbackRate: videoRef.value?.playbackRate || 1,
      clientTimeMs: Date.now(),
      ...patch,
    });
  }

  function seekBy(deltaMs) {
    if (videoRef.value) {
      applyingWatchState.value = true;
      videoRef.value.currentTime = Math.max(0, videoRef.value.currentTime + deltaMs / 1000);
      window.setTimeout(() => {
        applyingWatchState.value = false;
      }, 300);
    }
    sendWatchControl(deltaMs > 0 ? "forward" : "backward", { deltaMs: Math.abs(deltaMs) });
  }

  function applyWatchState(state) {
    if (state.roomId && state.roomId !== activeRoomId.value) return;
    applyingWatchState.value = true;
    if (state.action === "stop") {
      resetWatchSession();
      clearVideoContext();
      window.setTimeout(() => {
        applyingWatchState.value = false;
      }, 200);
      return;
    }

    const previousVideoId = video.fileId;
    watchSession.active = Boolean(state.videoId || state.videoUrl || state.action);
    watchSession.roomId = state.roomId || activeRoomId.value || "";
    watchSession.ownerId = state.updatedBy || "";
    watchSession.action = state.action || "";
    watchSession.videoId = state.videoId || "";
    watchSession.videoUrl = state.videoUrl || "";
    watchSession.updatedAtMs = state.updatedAtMs || Date.now();
    watchSession.clientTimeMs = state.clientTimeMs || 0;
    watchSession.durationMs = state.durationMs || 0;
    watchSession.playbackRate = state.playbackRate || 1;
    watchSession.isPlaying = Boolean(state.isPlaying);

    if (state.action === "load" || state.videoUrl) video.fileId = state.videoId || "";
    if (state.videoUrl) video.url = state.videoUrl;
    if (video.fileId !== previousVideoId) loadDanmaku();
    nextTick(() => {
      if (!videoRef.value) {
        applyingWatchState.value = false;
        return;
      }
      const target = (state.positionMs || 0) / 1000;
      if (Math.abs(videoRef.value.currentTime - target) > 1.2) videoRef.value.currentTime = target;
      videoRef.value.playbackRate = state.playbackRate || 1;
      if (state.isPlaying) videoRef.value.play().catch(() => {});
      else videoRef.value.pause();
      window.setTimeout(() => {
        applyingWatchState.value = false;
      }, 300);
    });
  }

  async function loadDanmaku() {
    if (!activeRoomId.value || !video.fileId || !token.value) {
      danmakuItems.value = [];
      return;
    }
    const data = await getDanmaku(token.value, activeRoomId.value, video.fileId).catch(() => ({ items: [] }));
    danmakuItems.value = mergeDanmakuItems(danmakuItems.value, data.items || []);
  }

  function onVideoTimeUpdate() {
    currentVideoTime.value = videoRef.value?.currentTime || 0;
  }

  function setVideoElement(el) {
    videoRef.value = el;
  }

  async function handleCreateRoom() {
    try {
      const data = await createRoom(token.value, roomForm);
      activeRoomId.value = data.roomId;
      roomForm.roomName = data.roomName || roomForm.roomName;
      openConversation(data.roomId, 2, data.roomName || "一起看房间");
      roomForm.inviteCodeDisplay = data.inviteCode || "";
      resetWatchSession();
      clearVideoContext();
      showMessage("房间已创建", "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function handleJoinRoom() {
    try {
      const data = await joinRoom(token.value, roomForm.inviteCode);
      activeRoomId.value = data.roomId || "";
      roomForm.roomName = data.roomName || roomForm.roomName;
      if (activeRoomId.value) {
        openConversation(activeRoomId.value, 2, roomForm.roomName || "一起看房间");
        resetWatchSession();
        clearVideoContext();
        if (wsConnected?.value) sendWatchControl("get_state");
      }
      showMessage("已加入房间", "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function handleInvite() {
    try {
      const data = await getInviteCode(token.value, activeRoomId.value);
      roomForm.inviteCodeDisplay = typeof data === "string" ? data : data.inviteCode;
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function handleUpload(event) {
    const file = event.target.files?.[0];
    if (!file) return;
    if (!ensureWatchEditable()) return;
    uploadName.value = file.name;
    try {
      const data = await chunkUpload.upload(token.value, file);
      Object.assign(video, { fileId: data.fileId, url: data.url, objectKey: data.objectKey, fileName: data.fileName });
      fileIdInput.value = data.fileId;
      loadDanmaku();
      showMessage("视频上传完成", "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  async function loadFile() {
    if (!ensureWatchEditable()) return;
    try {
      const data = await getFile(token.value, fileIdInput.value);
      Object.assign(video, { fileId: data.fileId, url: data.url, objectKey: data.objectKey, fileName: data.fileName });
      loadDanmaku();
      showMessage("视频已加载", "success");
    } catch (err) {
      showMessage(err.message);
    }
  }

  function loadVideoToRoom() {
    if (!activeRoomId.value) {
      showMessage("请先进入房间");
      return;
    }
    if (!canStartWatchSession.value) {
      if (!video.url) showMessage("请先加载视频");
      else showMessage(`当前由 ${watchOwnerLabel.value} 共享，请先结束后再发起`);
      return;
    }
    if (!video.url) {
      showMessage("请先加载视频");
      return;
    }
    sendWatchControl("load", { positionMs: 0 });
  }

  function stopWatchSession() {
    if (!canStopWatchSession.value) {
      showMessage("只有当前发起人可以结束共享");
      return;
    }
    sendWatchControl("stop");
  }

  function handleWsFrame(frame) {
    if (frame.op === "msg" && frame.payload?.hasVideoTime) {
      if (activeRoomId.value && frame.payload.conversationId === activeRoomId.value) {
        appendDanmakuItem({
          messageId: frame.payload.messageId,
          clientMsgId: frame.payload.clientMsgId,
          senderId: frame.payload.sendId,
          senderUsername: frame.payload.senderUsername,
          content: frame.payload.content,
          seq: frame.payload.seq,
          timeMs: frame.payload.videoTime || 0,
          sendTime: frame.payload.sendTime || Date.now(),
        });
      }
    }
    if (frame.op === "watch_video_sync") {
      applyWatchState(frame.payload);
    }
  }

  function resetRoomState() {
    activeRoomId.value = "";
    resetWatchSession();
    clearVideoContext();
  }

  return {
    roomForm,
    activeRoomId,
    fileIdInput,
    uploadName,
    video,
    watchSession,
    videoRef,
    danmakuItems,
    currentVideoTime,
    chunkUpload,
    applyingWatchState,
    visibleDanmaku,
    canControlWatchVideo,
    canStartWatchSession,
    canStopWatchSession,
    watchActionLabel,
    watchStatusLabel,
    watchOwnerLabel,
    resetWatchSession,
    clearVideoContext,
    ensureWatchEditable,
    appendDanmakuItem,
    mergeDanmakuItems,
    loadDanmaku,
    setVideoElement,
    onVideoTimeUpdate,
    applyWatchState,
    handleCreateRoom,
    handleJoinRoom,
    handleInvite,
    handleUpload,
    loadFile,
    loadVideoToRoom,
    stopWatchSession,
    sendWatchControl,
    seekBy,
    selectRoomConversation,
    handleWsFrame,
    resetRoomState,
  };
}
