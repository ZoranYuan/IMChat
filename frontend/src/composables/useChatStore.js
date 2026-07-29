import { computed, reactive } from "vue";
import {
  createFriendRequest,
  createRoom,
  getConversations,
  getFriendRequests,
  getFriends,
  getMessageHistory,
  joinRoom,
  loginUser,
  logoutUser,
  operateFriendRequest,
  registerUser,
} from "../api.js";
import {
  mockContacts,
  mockConversations,
  mockCurrentUser,
  mockFriendRequests,
  mockMessages,
} from "../mocks/chat.js";
import { createWsClient } from "../services/wsClient.js";
import { deleteMessages, readMessages, writeMessages } from "../services/messageDb.js";
import { useChunkUpload } from "./useChunkUpload.js";

const clone = (value) => JSON.parse(JSON.stringify(value));
const readJson = (key) => {
  try {
    return JSON.parse(localStorage.getItem(key) || sessionStorage.getItem(key) || "null");
  } catch {
    return null;
  }
};

const storedUser = readJson("im_user");
const storedToken = localStorage.getItem("im_token") || sessionStorage.getItem("im_token") || "";
const state = reactive({
  token: storedToken,
  currentUser: storedUser || clone(mockCurrentUser),
  conversations: clone(mockConversations),
  messages: clone(mockMessages),
  contacts: clone(mockContacts),
  friendRequests: clone(mockFriendRequests),
  activeConversationId: mockConversations[0].id,
  connection: "disconnected",
  loading: false,
  historyLoading: false,
  historyHasMore: {},
  historyCursor: {},
  readReceipts: {},
  dataSource: storedToken ? "api" : "preview",
});

const attachmentUpload = useChunkUpload();

const formatTime = (timestamp) => {
  if (!timestamp) return "";
  const date = new Date(Number(timestamp));
  const now = new Date();
  if (date.toDateString() === now.toDateString()) {
    return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit", hour12: false });
  }
  return `${date.getMonth() + 1}/${date.getDate()}`;
};

const contentPreview = (message) => {
  if (!message) return "暂无消息";
  const labels = { 2: "[图片]", 3: "[视频]", 4: "[表情]", 5: "[文件]" };
  return labels[message.cType] || message.content || "暂无消息";
};

const mediaContent = (cType, fileName) => {
  if (cType === 2) return "[图片]";
  if (cType === 3) return "[视频]";
  if (cType === 5) return `[文件] ${fileName}`;
  return fileName;
};

const normalizeConversation = (item) => ({
  id: item.conversationId,
  targetId: item.targetId,
  type: item.convType === 2 ? "group" : "direct",
  convType: item.convType,
  name: item.displayName || item.room?.roomName || item.peerUser?.remark || item.peerUser?.nickName || item.peerUser?.userName || "未命名会话",
  subtitle: contentPreview(item.lastMessage),
  time: formatTime(item.lastMessage?.sendTime),
  unread: Number(item.unread) || 0,
  pinned: false,
  muted: Boolean(item.isMuted),
  online: false,
  memberCount: item.room?.memberCount || 2,
  avatar: item.avatar || item.room?.avatar || item.peerUser?.avatar || "",
  avatarColor: item.convType === 2 ? "#2856a6" : "#2d7d68",
  tag: item.convType === 2 ? "群聊" : "联系人",
  description: item.room?.description || "",
});

const normalizeMessage = (item) => ({
  id: item.messageId,
  senderId: item.senderId || item.sendId,
  convType: Number(item.convType) || 1,
  senderName: item.senderUsername || "成员",
  seq: Number(item.seq) || 0,
  cType: Number(item.cType) || 1,
  type: Number(item.cType) === 2 ? "image" : Number(item.cType) === 3 ? "video" : Number(item.cType) === 5 ? "file" : "text",
  content: item.content || item.fileName || "",
  fileName: item.fileName || "",
  fileSize: item.fileSize || "",
  mediaUrl: item.mediaUrl || "",
  thumbUrl: item.thumbUrl || "",
  fileId: item.fileId || "",
  width: Number(item.width) || 0,
  height: Number(item.height) || 0,
  durationMs: Number(item.durationMs) || 0,
  time: formatTime(item.sendTime),
  sendTime: Number(item.sendTime) || Date.now(),
  status: item.status || "sent",
  clientMsgId: item.clientMsgId || "",
  avatarColor: item.senderId === state.currentUser.userId ? "#2856a6" : "#2d7d68",
});

const currentUserId = () => (state.currentUser && (state.currentUser.userId || state.currentUser.id)) || "";
const persistMessages = (conversationId) => {
  writeMessages(currentUserId(), conversationId, state.messages[conversationId] || []).catch(() => {});
};

const persistSession = (auth, remember) => {
  for (const storage of [localStorage, sessionStorage]) {
    storage.removeItem("im_token");
    storage.removeItem("im_user");
  }
  const storage = remember ? localStorage : sessionStorage;
  storage.setItem("im_token", auth.token);
  storage.setItem("im_user", JSON.stringify(auth));
};

const getImageDimensions = (file) => new Promise((resolve, reject) => {
  const url = URL.createObjectURL(file);
  const image = new Image();
  image.onload = () => {
    URL.revokeObjectURL(url);
    resolve({ width: image.naturalWidth || 0, height: image.naturalHeight || 0 });
  };
  image.onerror = (error) => {
    URL.revokeObjectURL(url);
    reject(error);
  };
  image.src = url;
});

const getVideoMetadata = (file) => new Promise((resolve, reject) => {
  const url = URL.createObjectURL(file);
  const video = document.createElement("video");
  video.preload = "metadata";
  video.onloadedmetadata = () => {
    URL.revokeObjectURL(url);
    resolve({
      width: video.videoWidth || 0,
      height: video.videoHeight || 0,
      durationMs: Number.isFinite(video.duration) ? Math.round(video.duration * 1000) : 0,
    });
  };
  video.onerror = (error) => {
    URL.revokeObjectURL(url);
    reject(error);
  };
  video.src = url;
});

const upsertIncomingMessage = (payload) => {
  const message = normalizeMessage(payload);
  const list = state.messages[payload.conversationId] || [];
  if (!list.some((item) => item.id === message.id)) list.push(message);
  state.messages[payload.conversationId] = list;
  persistMessages(payload.conversationId);
  let conversation = state.conversations.find((item) => item.id === payload.conversationId);
  if (!conversation) {
    conversation = {
      id: payload.conversationId,
      targetId: payload.convType === 2 ? payload.recvId : payload.sendId,
      type: payload.convType === 2 ? "group" : "direct",
      convType: payload.convType,
      name: payload.convType === 2 ? "新群聊" : payload.senderUsername || "新会话",
      subtitle: contentPreview(payload),
      time: formatTime(payload.sendTime),
      unread: state.activeConversationId === payload.conversationId ? 0 : 1,
      pinned: false,
      muted: false,
      online: false,
      memberCount: payload.convType === 2 ? 0 : 2,
      avatar: "",
      avatarColor: payload.convType === 2 ? "#2856a6" : "#2d7d68",
      tag: payload.convType === 2 ? "群聊" : "联系人",
      description: "",
    };
    state.conversations.unshift(conversation);
  }
  if (conversation) {
    conversation.subtitle = contentPreview(payload);
    conversation.time = formatTime(payload.sendTime);
    if (state.activeConversationId !== payload.conversationId) conversation.unread += 1;
  }
};

const handleMessageAck = (ack) => {
  Object.entries(state.messages).forEach(([conversationId, messages]) => {
    const pending = messages.find((item) => item.clientMsgId === ack.clientMsgId);
    if (!pending) return;
    pending.id = ack.messageId || pending.id;
    pending.status = ack.status === "failed" ? "failed" : "sent";
    pending.error = ack.extra || "";
    if (ack.sendTime) {
      pending.sendTime = ack.sendTime;
      pending.time = formatTime(ack.sendTime);
    }
    persistMessages(conversationId);
  });
};

const handleReadNotify = (receipt) => {
  state.readReceipts[receipt.conversationId] = receipt;
  const messages = state.messages[receipt.conversationId] || [];
  messages.forEach((message) => {
    if (message.senderId === state.currentUser.userId && message.seq && message.seq <= receipt.lastReadSeq) {
      message.status = "read";
    }
  });
};

const refreshConversationSnapshot = async () => {
  const items = await getConversations(state.token);
  const snapshot = (items || []).map(normalizeConversation);
  const localPinned = new Map(state.conversations.map((item) => [item.id, item.pinned]));
  snapshot.forEach((item) => {
    item.pinned = localPinned.get(item.id) || false;
  });
  state.conversations = snapshot;
};

const wsClient = createWsClient({
  onStateChange: (connection) => {
    state.connection = connection;
  },
  onOpen: ({ recovered }) => {
    if (recovered) refreshConversationSnapshot().catch(() => {
      state.connection = "error";
    });
  },
  onMessage: upsertIncomingMessage,
  onAck: handleMessageAck,
  onReadNotify: handleReadNotify,
});

const connectSocket = () => wsClient.connect(state.token);

export const hasChatAuth = () => Boolean(localStorage.getItem("im_token") || sessionStorage.getItem("im_token"));

export function useChatStore() {
  const activeConversation = computed(() =>
    state.conversations.find((item) => item.id === state.activeConversationId) || null,
  );
  const activeMessages = computed(() => state.messages[state.activeConversationId] || []);

  const authenticate = async ({ mode, phone, password, reconfirmPassword, remember }) => {
    const auth = mode === "register"
      ? await registerUser({ phone, password, reconfirmPassword })
      : await loginUser({ phone, password });
    state.token = auth.token;
    state.currentUser = auth;
    state.dataSource = "api";
    persistSession(auth, remember);
    return auth;
  };

  const loadWorkspace = async () => {
    if (!state.token) return;
    state.loading = true;
    try {
      const [conversations, friends, requests] = await Promise.all([
        getConversations(state.token),
        getFriends(state.token),
        getFriendRequests(state.token),
      ]);
      state.conversations = (conversations || []).map(normalizeConversation);
      state.contacts = (friends || []).map((item) => ({
        id: item.friendUserId,
        name: item.displayName || item.dsipalyName || item.friendUsername || "好友",
        username: item.friendUsername || "",
        avatar: item.friendAvatar || "",
        role: item.status === 1 ? "好友" : "联系人",
        online: false,
        avatarColor: "#2d7d68",
        conversationId: state.conversations.find((conversation) => conversation.targetId === item.friendUserId)?.id || "",
      }));
      state.friendRequests = (requests || []).filter((item) => item.status === 1).map((item) => ({
        id: item.requestId,
        fromUserId: item.fromUserId || "",
        name: item.fromDisplayName || item.fromUsername || item.fromUserId || "新联系人",
        note: item.message,
        time: formatTime(item.applyTime),
        avatarColor: "#357078",
      }));
      if (!state.conversations.some((item) => item.id === state.activeConversationId)) {
        state.activeConversationId = state.conversations[0]?.id || "";
      }
      connectSocket();
    } finally {
      state.loading = false;
    }
  };

  const loadHistory = async (conversationId, cursor = 0) => {
    if (!conversationId || !state.token) return;
    state.historyLoading = true;
    try {
      const data = await getMessageHistory(state.token, conversationId, cursor, 30);
      const incoming = (data?.messages || []).map(normalizeMessage);
      const existing = state.messages[conversationId] || [];
      const incomingIds = new Set(incoming.map((item) => item.id));
      state.messages[conversationId] = cursor
        ? [...incoming, ...existing.filter((item) => !incomingIds.has(item.id))]
        : [...incoming, ...existing.filter((item) => !incomingIds.has(item.id))].sort((a, b) => (a.seq || a.sendTime) - (b.seq || b.sendTime));
      persistMessages(conversationId);
      state.historyCursor[conversationId] = data?.nextCursor || 0;
      state.historyHasMore[conversationId] = Boolean(data?.hasMore);
    } finally {
      state.historyLoading = false;
    }
  };

  const selectConversation = async (id) => {
    state.activeConversationId = id;
    const conversation = state.conversations.find((item) => item.id === id);
    if (conversation) conversation.unread = 0;
    if (state.token) {
      const cached = await readMessages(currentUserId(), id).catch(() => []);
      if (state.activeConversationId === id) state.messages[id] = cached.map(normalizeMessage);
      await loadHistory(id, 0);
    }
  };

  const loadOlderMessages = () => {
    const id = state.activeConversationId;
    return loadHistory(id, state.historyCursor[id] || 0);
  };

  const appendOutgoingMessage = (conversation, payload) => {
    const message = {
      id: payload.clientMsgId,
      clientMsgId: payload.clientMsgId,
      senderId: state.currentUser.userId,
      senderName: state.currentUser.nickName || state.currentUser.username || "我",
      content: payload.content,
      cType: payload.cType,
      convType: payload.convType,
      type: payload.cType === 2 ? "image" : payload.cType === 3 ? "video" : payload.cType === 5 ? "file" : "text",
      fileId: payload.fileId || "",
      fileName: payload.fileName || "",
      fileSize: payload.fileSize || 0,
      mediaUrl: payload.mediaUrl || "",
      thumbUrl: payload.thumbUrl || "",
      width: payload.width || 0,
      height: payload.height || 0,
      durationMs: payload.durationMs || 0,
      time: formatTime(Date.now()),
      sendTime: Date.now(),
      status: "sending",
      avatarColor: "#2856a6",
    };
    if (!state.messages[conversation.id]) state.messages[conversation.id] = [];
    state.messages[conversation.id].push(message);
    persistMessages(conversation.id);
    conversation.subtitle = payload.cType === 1 ? `你：${payload.content}` : contentPreview(payload);
    conversation.time = "刚刚";
    return message;
  };

  const sendMessage = (content) => {
    const text = content.trim();
    const conversation = activeConversation.value;
    if (!text || !conversation) return null;
    if (!wsClient.isConnected()) throw new Error("实时连接尚未建立，请稍后重试。" );
    const clientMsgId = crypto.randomUUID?.() || `message-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    const payload = {
      clientMsgId,
      recvId: conversation.targetId,
      convType: conversation.convType || (conversation.type === "group" ? 2 : 1),
      cType: 1,
      content: text,
    };
    if (!wsClient.sendMessage(payload)) throw new Error("消息发送失败，请重新连接后重试。" );
    return appendOutgoingMessage(conversation, payload);
  };

  const sendAttachment = async (file, cType) => {
    const conversation = activeConversation.value;
    if (!file || !conversation) return null;
    if (![2, 3, 5].includes(cType)) throw new Error("不支持的附件类型。" );
    if (!wsClient.isConnected()) throw new Error("实时连接尚未建立，请稍后重试。" );

    const uploaded = await attachmentUpload.upload(state.token, file);
    const mediaMeta = cType === 2
      ? await getImageDimensions(file).catch(() => ({ width: 0, height: 0, durationMs: 0 }))
      : cType === 3
        ? await getVideoMetadata(file).catch(() => ({ width: 0, height: 0, durationMs: 0 }))
        : { width: 0, height: 0, durationMs: 0 };
    const clientMsgId = crypto.randomUUID?.() || `message-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    const payload = {
      clientMsgId,
      recvId: conversation.targetId,
      convType: conversation.convType || (conversation.type === "group" ? 2 : 1),
      cType,
      content: mediaContent(cType, uploaded.fileName || file.name),
      mediaUrl: uploaded.url || "",
      thumbUrl: cType === 2 ? uploaded.url || "" : "",
      fileId: uploaded.fileId || "",
      thumbFileId: "",
      fileName: uploaded.fileName || file.name,
      fileSize: uploaded.size || file.size,
      width: mediaMeta.width,
      height: mediaMeta.height,
      durationMs: mediaMeta.durationMs,
      hasVideoTime: false,
    };
    if (!wsClient.sendMessage(payload)) throw new Error("文件已上传，但实时连接已断开，请重新发送。" );
    return appendOutgoingMessage(conversation, payload);
  };

  const sendReadAck = () => {
    const conversation = activeConversation.value;
    const last = activeMessages.value.at(-1);
    if (!conversation || !last?.seq || !wsClient.isConnected()) return;
    wsClient.sendReadAck({
      conversationId: conversation.id,
      lastReadSeq: last.seq,
      senderId: last.senderId,
    });
  };

  const togglePinned = (id) => {
    const item = state.conversations.find((conversation) => conversation.id === id);
    if (item) item.pinned = !item.pinned;
    state.conversations.sort((a, b) => Number(b.pinned) - Number(a.pinned));
    return item?.pinned;
  };

  const toggleMuted = (id) => {
    const item = state.conversations.find((conversation) => conversation.id === id);
    if (item) item.muted = !item.muted;
    return item?.muted;
  };

  const clearConversation = (id) => {
    state.messages[id] = [];
    deleteMessages(currentUserId(), id).catch(() => {});
  };

  const handleFriendRequest = async (request, accepted) => {
    await operateFriendRequest(state.token, {
      requestId: request.id,
      fromUserId: request.fromUserId,
      action: accepted ? 1 : 2,
    });
    state.friendRequests = state.friendRequests.filter((item) => item.id !== request.id);
    if (accepted) await loadWorkspace();
  };

  const addFriendRequest = (form) => createFriendRequest(state.token, form);

  const startConversation = async (contact) => {
    let conversation = state.conversations.find((item) => item.targetId === contact.id || item.id === contact.conversationId);
    if (!conversation) {
      const currentUserId = state.currentUser.userId;
      if (!currentUserId) throw new Error("当前用户信息不完整，请重新登录。" );
      const conversationId = [currentUserId, contact.id].sort().reverse().join("_");
      conversation = {
        id: conversationId,
        targetId: contact.id,
        type: "direct",
        convType: 1,
        name: contact.name,
        subtitle: "可以开始聊天了",
        time: "",
        unread: 0,
        pinned: false,
        muted: false,
        online: contact.online,
        memberCount: 2,
        avatar: contact.avatar || "",
        avatarColor: contact.avatarColor || "#2d7d68",
        tag: "联系人",
        description: "",
      };
      state.conversations.unshift(conversation);
      state.messages[conversationId] = [];
      state.activeConversationId = conversationId;
      return conversation;
    }
    await selectConversation(conversation.id);
    return conversation;
  };

  const createChatRoom = async (form) => {
    const room = await createRoom(state.token, form);
    await loadWorkspace();
    return room;
  };

  const joinChatRoom = async (inviteCode) => {
    const room = await joinRoom(state.token, inviteCode);
    await loadWorkspace();
    return room;
  };

  const retryConnection = () => connectSocket();

  const logout = async () => {
    wsClient.disconnect();
    try {
      if (state.token) await logoutUser(state.token);
    } finally {
      state.token = "";
      for (const storage of [localStorage, sessionStorage]) {
        storage.removeItem("im_token");
        storage.removeItem("im_user");
      }
    }
  };

  return {
    state,
    activeConversation,
    activeMessages,
    authenticate,
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
    uploadStatus: attachmentUpload.status,
    uploadProgress: attachmentUpload.progress,
    uploadIsActive: attachmentUpload.isUploading,
    pauseUpload: attachmentUpload.pause,
    resumeUpload: attachmentUpload.resume,
    cancelUpload: attachmentUpload.cancel,
    logout,
  };
}
