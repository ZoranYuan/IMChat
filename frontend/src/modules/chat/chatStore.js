import { defineStore } from "pinia";
import { computed, reactive } from "vue";
import {
  createFriendRequest,
  createRoom,
  getAttachmentAccessURLs,
  getConversations,
  getFriendRequests,
  getFriends,
  getMessageHistory,
  getMessagesBySeqs,
  joinRoom,
  loginUser,
  logoutUser,
  operateFriendRequest,
  refreshSession,
  registerUser,
  syncMessages,
  updateUserProfile,
} from "../../api.js";
import { useChunkUpload } from "../../composables/useChunkUpload.js";
import { MessageType, UploadableMessageTypes } from "../../constants/message.js";
import {
  clearConversations,
  insertConversations,
  queryConversationsByUserId,
  replaceConversations,
  updateConversationLastReadSeq,
} from "../../db/services/conversationService.js";
import {
  clearMessages,
  deleteMessagesByConversation,
  insertMessages,
  queryMessagesByCursor,
} from "../../db/services/messageService.js";
import {
  mockContacts,
  mockFriendRequests,
} from "../../mocks/chat.js";
import { createWsClient } from "../../services/wsClient.js";
import { createAttachmentResolver } from "./services/attachmentService.js";
import {
  authUserProfile,
  clone,
  initialUser,
  persistSession,
} from "./services/authService.js";
import { createMessageSyncService } from "./services/messageSyncService.js";
import {
  applyIncomingMessageToConversation,
  applySyncedMessagesToConversation,
  createDirectConversation,
  findConversation,
  sortConversations,
} from "./utils/conversationState.js";
import {
  createClientMessageId,
  createConversationPreview,
  createOutgoingMessage,
  getImageDimensions,
  getVideoMetadata,
} from "./utils/messageComposer.js";
import {
  confirmedMessages,
  latestConfirmedMessage,
  mergeMessageLists,
  normalizeMessage,
} from "./utils/messageMerge.js";

const HISTORY_PAGE_SIZE = 30;
const THEME_CONTACT_AVATAR = "#8b72d6";

const state = reactive({
  authenticated: false,
  currentUser: initialUser(),
  conversations: [],
  messages: {},
  contacts: clone(mockContacts),
  friendRequests: clone(mockFriendRequests),
  activeConversationId: "",
  connection: "disconnected",
  loading: false,
  historyLoading: false,
  historyHasMore: {},
  historyCursor: {},
  readReceipts: {},
});

const attachmentResolver = createAttachmentResolver(getAttachmentAccessURLs);
const attachmentUpload = useChunkUpload();
const pendingMessages = new Map();
const historyRequests = new Map();

let sessionGeneration = 0;
let cacheClearPromise = Promise.resolve();
let authBootstrapPromise = null;
let historyRequestSequence = 0;

/** 返回当前登录用户 ID，作为本地缓存和会话请求的隔离边界。 */
const currentUserId = () => state.currentUser?.userId || state.currentUser?.id || "";

/** 捕获当前认证代次，防止旧请求在切换用户后回写新会话数据。 */
const captureSession = () => ({
  generation: sessionGeneration,
  userId: currentUserId(),
});
/** 判断异步操作对应的用户会话是否仍然有效。 */
const isSessionActive = ({ generation, userId }) => (
  state.authenticated
  && generation === sessionGeneration
  && userId !== ""
  && userId === currentUserId()
);

const formatTime = (timestamp) => {
  if (!timestamp) return "";
  const date = new Date(Number(timestamp));
  const now = new Date();
  if (date.toDateString() === now.toDateString()) {
    return date.toLocaleTimeString("zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  }
  return `${date.getMonth() + 1}/${date.getDate()}`;
};

/** 只持久化已经获得服务端 seq 和 messageId 的确认消息。 */
const persistConfirmedMessages = (messages) => (
  insertMessages(confirmedMessages(messages))
);

/** 将会话快照写入当前用户的本地会话表。 */
const persistConversation = (conversation, session = captureSession()) => {
  if (!conversation || !isSessionActive(session)) return Promise.resolve();
  return insertConversations(session.userId, [conversation]).catch(() => { });
};

const messageSync = createMessageSyncService({
  getHistory: getMessageHistory,
  getMessagesBySeqs,
  syncMessages,
  queryMessagesByCursor,
  insertMessages,
  canContinue: () => state.authenticated && currentUserId() !== "",
});

/** 合并消息、解析附件展示信息，并按需持久化消息本体。 */
const mergeMessages = async (
  conversationId,
  incomingItems,
  { persist = true, session = captureSession() } = {},
) => {
  const incoming = (incomingItems || []).map(normalizeMessage);
  const mergedMessages = mergeMessageLists(
    state.messages[conversationId] || [],
    incoming,
  );
  await attachmentResolver.resolve(mergedMessages);
  if (!isSessionActive(session)) {
    return state.messages[conversationId] || [];
  }
  state.messages[conversationId] = mergedMessages;
  if (persist && isSessionActive(session)) await persistConfirmedMessages(incoming);
  return state.messages[conversationId];
};

/** 按最新消息时间重新排列内存中的会话列表。 */
const updateConversationList = () => {
  state.conversations = sortConversations(state.conversations);
};

/** 处理 WebSocket 推送的 MessageEvent，按 seq/clientMsgId 去重并更新会话状态。 */
const upsertIncomingMessage = (payload) => {
  const session = captureSession();
  const conversationId = payload?.conversationId || "";
  const message = normalizeMessage(payload);
  const conversation = findConversation(state.conversations, conversationId);

  // 会话快照是会话的唯一来源；未知会话的消息不能直接在本地伪造会话。
  if (
    !isSessionActive(session)
    || !conversation
    || !message.messageId
    || Number(message.seq) <= 0
  ) return;

  const active = state.activeConversationId === conversationId;
  if (active) {
    state.messages[conversationId] = mergeMessageLists(
      state.messages[conversationId] || [],
      [message],
    );
    const storedMessage = state.messages[conversationId].find((item) => (
      (message.seq > 0 && Number(item.seq) === message.seq)
      || (message.clientMsgId && item.clientMsgId === message.clientMsgId)
    ));
    if (storedMessage) attachmentResolver.resolve([storedMessage]).catch(() => { });
  }

  persistConfirmedMessages([message]).catch(() => { });
  applyIncomingMessageToConversation(conversation, message, { active });
  updateConversationList();
  persistConversation(conversation, session);
};

/** 将消息 ACK 应用到发送中的本地消息，并用服务端 seq 完成确认。 */
const handleMessageAck = (ack) => {
  const pending = pendingMessages.get(ack?.clientMsgId);
  if (!pending) return;

  const conversationId = ack.conversationId || pending.conversationId;
  const updated = normalizeMessage({
    ...pending,
    messageId: ack.messageId || pending.messageId || "",
    conversationId,
    seq: Number(ack.seq) || pending.seq || 0,
    attachmentId: ack.attachmentId || pending.attachmentId || "",
    sendTime: Number(ack.sendTime) || pending.sendTime,
    status: ack.status === "failed" ? "failed" : "sent",
    error: ack.extra || "",
  });
  pendingMessages.delete(ack.clientMsgId);

  state.messages[conversationId] = mergeMessageLists(
    state.messages[conversationId] || [],
    [updated],
  );
  const storedMessage = state.messages[conversationId].find((item) => (
    (updated.seq > 0 && Number(item.seq) === updated.seq)
    || (updated.clientMsgId && item.clientMsgId === updated.clientMsgId)
  ));
  if (storedMessage) attachmentResolver.resolve([storedMessage]).catch(() => { });
  persistConfirmedMessages([updated]).catch(() => { });

  const conversation = findConversation(state.conversations, conversationId);
  if (conversation && updated.seq > 0) {
    applyIncomingMessageToConversation(conversation, updated, {
      active: state.activeConversationId === conversationId,
    });
    updateConversationList();
    persistConversation(conversation);
  }
};

/** 处理对方已读通知，更新本地已读回执和发送消息状态。 */
const handleReadNotify = (receipt) => {
  const conversationId = receipt?.conversationId || "";
  if (!conversationId) return;

  const previous = state.readReceipts[conversationId];
  if (previous && Number(previous.lastReadSeq) >= Number(receipt.lastReadSeq)) return;
  state.readReceipts[conversationId] = receipt;

  const userId = currentUserId();
  for (const message of state.messages[conversationId] || []) {
    if (
      message.senderId === userId
      && Number(message.seq) > 0
      && Number(message.seq) <= Number(receipt.lastReadSeq)
    ) message.status = "read";
  }
};

/** 从服务端刷新会话快照，并替换当前用户的本地会话数据。 */
const refreshConversationSnapshot = async (session = captureSession()) => {
  const items = await getConversations();
  if (!isSessionActive(session)) return [];

  const conversations = sortConversations(items || []);
  state.conversations = conversations;
  if (!findConversation(conversations, state.activeConversationId)) {
    state.activeConversationId = "";
  }
  await replaceConversations(session.userId, conversations).catch(() => { });
  return conversations;
};

/** 按会话已读边界同步缺失消息，并更新内存和本地消息缓存。 */
const syncConversation = async (conversationId, { force = false } = {}) => {
  const session = captureSession();
  if (!conversationId || !isSessionActive(session)) return;

  const conversation = findConversation(state.conversations, conversationId);
  if (!conversation) return;

  const lastReadSeq = Number(conversation.lastReadSeq) || 0;
  const latestSeq = Number(conversation.latestSeq) || 0;
  if (!force && latestSeq <= lastReadSeq) return;

  const scope = `${session.userId}:${session.generation}`;
  const data = await messageSync.syncAfter(conversationId, lastReadSeq, scope);
  if (!isSessionActive(session)) return;

  const messages = data?.messages || [];
  messageSync.invalidateHistory(conversationId, scope);
  if (state.activeConversationId === conversationId) {
    await mergeMessages(conversationId, messages, { session });
  } else {
    await persistConfirmedMessages(messages);
  }

  if (messages.length) {
    const currentConversation = findConversation(state.conversations, conversationId);
    if (!currentConversation || !isSessionActive(session)) return;
    applySyncedMessagesToConversation(currentConversation, messages, {
      active: state.activeConversationId === conversationId,
    });
    updateConversationList();
    await persistConversation(currentConversation, session);
  }
};

const wsClient = createWsClient({
  onStateChange: (connection) => {
    state.connection = connection;
  },
  onOpen: ({ recovered }) => {
    if (!recovered) return;
    const session = captureSession();
    refreshConversationSnapshot(session)
      .then((conversations) => Promise.all(
        conversations.map((conversation) => syncConversation(conversation.conversationId)),
      ))
      .catch(() => { });
  },
  onMessage: upsertIncomingMessage,
  onAck: handleMessageAck,
  onReadNotify: handleReadNotify,
  onRoomMessageNotice: (notice) => {
    syncConversation(notice?.conversationId, { force: true }).catch(() => { });
  },
});

const connectSocket = () => {
  if (state.authenticated && !wsClient.isConnected()) wsClient.connect(true);
};

const clearAuthState = () => {
  sessionGeneration += 1;
  pendingMessages.clear();
  wsClient.disconnect();
  state.authenticated = false;
  state.currentUser = null;
  state.connection = "disconnected";
  state.loading = false;
  state.historyLoading = false;
  state.conversations = [];
  state.messages = {};
  state.contacts = [];
  state.friendRequests = [];
  state.activeConversationId = "";
  state.historyHasMore = {};
  state.historyCursor = {};
  state.readReceipts = {};
  attachmentResolver.clear();
  historyRequests.clear();

  for (const storage of [localStorage, sessionStorage]) {
    storage.removeItem("im_user");
  }

  cacheClearPromise = Promise.all([
    clearMessages(),
    clearConversations(),
  ]).catch(() => { });
  return cacheClearPromise;
};

if (typeof window !== "undefined") {
  window.addEventListener("auth:expired", () => {
    clearAuthState();
  });
}

export const hasChatAuth = () => state.authenticated;

export const ensureChatAuth = async () => {
  if (state.authenticated) return true;
  if (!authBootstrapPromise) {
    const generation = sessionGeneration;
    authBootstrapPromise = refreshSession()
      .then((auth) => {
        if (generation !== sessionGeneration) return false;
        state.authenticated = true;
        state.currentUser = authUserProfile(auth);
        persistSession(auth, Boolean(localStorage.getItem("im_user")));
        return true;
      })
      .catch(async () => {
        await clearAuthState();
        return false;
      })
      .finally(() => {
        authBootstrapPromise = null;
      });
  }
  return authBootstrapPromise;
};

export const useChatStore = defineStore("chat", () => {
  const activeConversation = computed(() => (
    findConversation(state.conversations, state.activeConversationId)
  ));
  const activeMessages = computed(() => state.messages[state.activeConversationId] || []);

  const authenticate = async ({ mode, account, password, reconfirmPassword, remember }) => {
    if (mode === "register") {
      return registerUser({ phone: account, password, reconfirmPassword });
    }

    const auth = await loginUser({ account, password });
    await cacheClearPromise;
    state.authenticated = true;
    state.currentUser = authUserProfile(auth);
    persistSession(auth, remember);
    return auth;
  };

  const updateProfile = async (payload) => {
    const profile = await updateUserProfile(payload);
    state.currentUser = authUserProfile({ ...state.currentUser, ...profile });
    persistSession(state.currentUser, Boolean(localStorage.getItem("im_user")));
    return state.currentUser;
  };

  const loadWorkspace = async () => {
    if (!state.authenticated) return;
    const session = captureSession();
    const pendingClear = cacheClearPromise;
    state.loading = true;

    try {
      await pendingClear;
      if (!isSessionActive(session)) return;

      const [cachedConversations, remoteResult, friends, requests] = await Promise.all([
        queryConversationsByUserId(session.userId).catch(() => []),
        getConversations()
          .then((value) => ({ ok: true, value: value || [] }))
          .catch((error) => ({ ok: false, error })),
        getFriends(),
        getFriendRequests(),
      ]);
      if (!isSessionActive(session)) return;
      if (!remoteResult.ok && !cachedConversations.length) throw remoteResult.error;

      const conversations = sortConversations(
        remoteResult.ok ? remoteResult.value : cachedConversations,
      );
      state.conversations = conversations;
      if (remoteResult.ok) {
        await replaceConversations(session.userId, conversations).catch(() => { });
      }

      const conversationByTargetId = new Map(
        conversations.map((conversation) => [conversation.targetId, conversation]),
      );
      state.contacts = (friends || []).map((item) => ({
        id: item.friendUserId,
        name: item.displayName || item.friendUsername || "好友",
        username: item.friendUsername || "",
        avatar: item.friendAvatar || "",
        online: false,
        avatarColor: THEME_CONTACT_AVATAR,
        conversationId: conversationByTargetId.get(item.friendUserId)?.conversationId || "",
      }));
      state.friendRequests = (requests || []).filter((item) => item.status === 1).map((item) => ({
        id: item.requestId,
        applicantUserId: item.applicantUserId || "",
        name: item.applicantNickName || "新联系人",
        note: item.message,
        time: formatTime(item.applyTime),
        avatarColor: "#8069a8",
      }));
      if (!findConversation(state.conversations, state.activeConversationId)) {
        state.activeConversationId = "";
      }

      connectSocket();
      Promise.all(conversations.map((conversation) => (
        syncConversation(conversation.conversationId)
      ))).catch(() => { });
    } finally {
      if (isSessionActive(session)) state.loading = false;
    }
  };

  const loadHistory = async (conversationId, cursor = 0) => {
    const session = captureSession();
    if (!conversationId || !isSessionActive(session)) return;

    const requestId = ++historyRequestSequence;
    historyRequests.set(conversationId, requestId);
    state.historyLoading = true;
    try {
      const conversation = findConversation(state.conversations, conversationId);
      const data = await messageSync.loadHistoryPage({
        conversationId,
        cursor,
        limit: HISTORY_PAGE_SIZE,
        latestSeq: Number(conversation?.latestSeq) || 0,
        scope: `${session.userId}:${session.generation}`,
        requestCanContinue: () => isSessionActive(session),
      });
      if (!isSessionActive(session) || state.activeConversationId !== conversationId) return;

      // 历史服务已经负责写入远端页面；这里仅合并到当前会话内存。
      await mergeMessages(conversationId, data.messages, { persist: false, session });
      state.historyCursor[conversationId] = data.nextCursor;
      state.historyHasMore[conversationId] = data.hasMore;
    } finally {
      if (historyRequests.get(conversationId) === requestId) {
        historyRequests.delete(conversationId);
      }
      if (!historyRequests.has(state.activeConversationId)) state.historyLoading = false;
    }
  };

  const selectConversation = async (conversationId) => {
    state.activeConversationId = conversationId;
    const conversation = findConversation(state.conversations, conversationId);
    if (conversation) conversation.unread = 0;
    if (state.authenticated) await loadHistory(conversationId, 0);
  };

  const loadOlderMessages = async () => {
    const conversationId = state.activeConversationId;
    const cursor = state.historyCursor[conversationId];
    if (!conversationId || cursor === -1 || state.historyLoading) return;
    return loadHistory(conversationId, cursor);
  };

  const appendOutgoingMessage = (conversation, payload) => {
    const message = createOutgoingMessage(conversation, payload, state.currentUser);
    if (!state.messages[conversation.conversationId]) {
      state.messages[conversation.conversationId] = [];
    }
    state.messages[conversation.conversationId] = mergeMessageLists(
      state.messages[conversation.conversationId],
      [message],
    );
    pendingMessages.set(payload.clientMsgId, message);
    conversation.lastMessage = createConversationPreview(
      conversation,
      message,
      state.currentUser,
    );
    updateConversationList();
    persistConversation(conversation);
    return message;
  };

  const sendMessage = (content) => {
    const text = typeof content === "string" ? content.trim() : "";
    const conversation = activeConversation.value;
    if (!text || !conversation) return null;
    if (!wsClient.isConnected()) throw new Error("实时连接尚未建立，请稍后重试。");

    const payload = {
      clientMsgId: createClientMessageId(),
      recvId: conversation.targetId,
      convType: conversation.convType,
      cType: MessageType.TEXT,
      content: text,
    };
    if (!wsClient.sendMessage(payload)) throw new Error("消息发送失败，请重新连接后重试。");
    return appendOutgoingMessage(conversation, payload);
  };

  const sendAttachment = async (file, cType) => {
    if (!file) return null;
    const type = Number(cType);
    if (!UploadableMessageTypes.includes(type)) throw new Error("不支持的附件类型。");
    const conversation = activeConversation.value;
    if (!conversation) return null;
    if (!state.authenticated) throw new Error("登录状态已失效，请重新登录。");
    if (!wsClient.isConnected()) throw new Error("实时连接尚未建立，请稍后重试。");

    const metadata = type === MessageType.IMAGE
      ? await getImageDimensions(file)
      : type === MessageType.VIDEO
        ? await getVideoMetadata(file)
        : {};
    const uploaded = await attachmentUpload.upload(file);
    const payload = {
      clientMsgId: createClientMessageId(),
      recvId: conversation.targetId,
      convType: conversation.convType,
      cType: type,
      fileId: uploaded?.fileId || "",
      ...metadata,
    };
    if (type === MessageType.VIDEO && metadata.durationMs) {
      payload.durationMs = metadata.durationMs;
    }
    if (!payload.fileId) throw new Error("上传成功但未返回文件标识。");
    if (!wsClient.sendMessage(payload)) throw new Error("消息发送失败，请重新连接后重试。");
    return appendOutgoingMessage(conversation, payload);
  };

  const sendReadAck = () => {
    const conversation = activeConversation.value;
    const confirmedLastMessage = latestConfirmedMessage(activeMessages.value);
    const latestSeq = Number(conversation?.latestSeq) || 0;
    const lastReadSeq = Number(conversation?.lastReadSeq) || 0;
    const readSeq = Number(confirmedLastMessage?.seq) || 0;
    if (
      !conversation
      || latestSeq <= lastReadSeq
      || readSeq <= lastReadSeq
      || !wsClient.isConnected()
    ) return;

    const sent = wsClient.sendReadAck({
      conversationId: conversation.conversationId,
      lastReadSeq: readSeq,
    });
    if (!sent) return;

    conversation.lastReadSeq = Math.max(lastReadSeq, readSeq);
    conversation.unread = Math.max(latestSeq - conversation.lastReadSeq, 0);
    updateConversationLastReadSeq({
      userId: currentUserId(),
      conversationId: conversation.conversationId,
      lastReadSeq: conversation.lastReadSeq,
    }).catch(() => { });
  };

  const clearConversation = async (conversationId) => {
    if (!conversationId) return;
    state.messages[conversationId] = [];
    state.historyCursor[conversationId] = 0;
    state.historyHasMore[conversationId] = false;
    const session = captureSession();
    messageSync.invalidateHistory(
      conversationId,
      `${session.userId}:${session.generation}`,
    );
    for (const [clientMsgId, message] of pendingMessages) {
      if (message.conversationId === conversationId) pendingMessages.delete(clientMsgId);
    }
    await deleteMessagesByConversation(conversationId);
  };

  const handleFriendRequest = async (request, accepted) => {
    await operateFriendRequest({
      requestId: request.id,
      action: accepted ? 1 : 2,
    });
    state.friendRequests = state.friendRequests.filter((item) => item.id !== request.id);
    if (accepted) await loadWorkspace();
  };

  const addFriendRequest = (form) => createFriendRequest(form);

  const startConversation = async (contact) => {
    let conversation = state.conversations.find((item) => (
      item.targetId === contact.id || item.conversationId === contact.conversationId
    ));
    if (!conversation) {
      const userId = currentUserId();
      if (!userId) throw new Error("当前用户信息不完整，请重新登录。");
      conversation = createDirectConversation(userId, contact);
      state.conversations = sortConversations([conversation, ...state.conversations]);
      state.messages[conversation.conversationId] = [];
      state.activeConversationId = conversation.conversationId;
      await persistConversation(conversation);
      return conversation;
    }

    await selectConversation(conversation.conversationId);
    return conversation;
  };

  const createChatRoom = async (form) => {
    const room = await createRoom(form);
    await loadWorkspace();
    return room;
  };

  const joinChatRoom = async (inviteCode) => {
    const room = await joinRoom(inviteCode);
    await loadWorkspace();
    return room;
  };

  const retryConnection = () => wsClient.reconnect();

  const logout = async () => {
    try {
      if (state.authenticated) await logoutUser();
    } finally {
      await clearAuthState();
    }
  };

  return {
    state,
    activeConversation,
    activeMessages,
    authenticate,
    updateProfile,
    loadWorkspace,
    selectConversation,
    loadOlderMessages,
    sendMessage,
    sendAttachment,
    sendReadAck,
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
});
