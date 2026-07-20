import { nextTick, ref, watch } from "vue";
import { getConversations, getHistoryMessages, uploadFile } from "../api";

export function useConversation({ token, currentUser, showMessage, sendFrame }) {
  const conversations = ref([]);
  const activeConversation = ref(null);
  const messages = ref([]);
  const readReceipts = ref({});
  const conversationLoading = ref(false);
  const messageText = ref("");
  const messageList = ref(null);
  const pendingLocalMessages = [];
  const conversationMessageCacheKey = "im_active_conversation_messages";
  let conversationLoadSeq = 0;

  function getConversationReadState(conversationId) {
    if (!conversationId) {
      return { lastReadSeq: 0, readers: [] };
    }
    return readReceipts.value[conversationId] || { lastReadSeq: 0, readers: [] };
  }

  const activeReadState = ref({ lastReadSeq: 0, readers: [] });

  function getConversationIndex(conversationId) {
    return conversations.value.findIndex((item) => item.conversationId === conversationId);
  }

  function upsertConversationPreview(conversationId, patch = {}, moveToTop = false) {
    if (!conversationId) return null;
    const index = getConversationIndex(conversationId);
    const existing = index >= 0 ? conversations.value[index] : null;
    const nextUnread = (() => {
      if (typeof patch.unread === "number") return Math.max(0, patch.unread);
      if (typeof patch.unreadDelta === "number") return Math.max(0, (existing?.unread || 0) + patch.unreadDelta);
      if (patch.resetUnread) return 0;
      return existing?.unread || 0;
    })();

    const nextItem = {
      conversationId,
      displayName: existing?.displayName || "会话",
      avatar: existing?.avatar || "",
      unread: nextUnread,
      latestMessage: patch.latestMessage ?? existing?.latestMessage ?? null,
      convType: patch.convType ?? existing?.convType ?? patch.latestMessage?.convType ?? 2,
      targetId: patch.targetId ?? existing?.targetId ?? conversationId,
      peerUser: patch.peerUser ?? existing?.peerUser ?? null,
      room: patch.room ?? existing?.room ?? null,
      isMuted: patch.isMuted ?? existing?.isMuted ?? false,
    };

    if (Object.prototype.hasOwnProperty.call(patch, "displayName")) {
      nextItem.displayName = patch.displayName || nextItem.displayName;
    }
    if (Object.prototype.hasOwnProperty.call(patch, "avatar")) {
      nextItem.avatar = patch.avatar || "";
    }

    if (existing) {
      Object.assign(existing, nextItem);
      conversations.value = moveToTop
        ? [existing, ...conversations.value.filter((item) => item.conversationId !== conversationId)]
        : conversations.value.map((item) => (item.conversationId === conversationId ? existing : item));
      if (activeConversation.value?.conversationId === conversationId) activeConversation.value = existing;
      return existing;
    }

    conversations.value = moveToTop ? [nextItem, ...conversations.value] : [...conversations.value, nextItem];
    if (activeConversation.value?.conversationId === conversationId) activeConversation.value = nextItem;
    return nextItem;
  }

  function cloneCachedMessages(items = []) {
    return items.map((item) => ({ ...item }));
  }

  function readConversationMessagesCache() {
    try {
      const raw = localStorage.getItem(conversationMessageCacheKey);
      if (!raw) return null;
      const parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== "object") return null;
      if (!parsed.conversationId || !Array.isArray(parsed.messages)) return null;
      return parsed;
    } catch {
      return null;
    }
  }

  function getConversationMessagesCache(conversationId) {
    const cached = readConversationMessagesCache();
    if (!cached || cached.conversationId !== conversationId) return null;
    return cloneCachedMessages(cached.messages);
  }

  function setConversationMessagesCache(conversationId, items) {
    if (!conversationId) return;
    try {
      localStorage.setItem(
        conversationMessageCacheKey,
        JSON.stringify({
          conversationId,
          messages: cloneCachedMessages(items || []),
        }),
      );
    } catch {
      // ignore storage quota / serialization issues
    }
  }

  function clearConversationMessagesCache() {
    localStorage.removeItem(conversationMessageCacheKey);
  }

  function messageDedupKey(message) {
    return (
      message?.messageId ||
      message?.clientMsgId ||
      `${message?.conversationId || ""}:${message?.seq || 0}:${message?.sendTime || 0}:${message?.content || ""}`
    );
  }

  function getMessagePreviewText(message) {
    if (!message) return "暂无消息";
    switch (Number(message.cType) || 0) {
      case 2:
        return message.content && message.content !== "[图片]" ? message.content : "[图片]";
      case 3:
        return message.content && message.content !== "[视频]" ? message.content : "[视频]";
      case 4:
        return message.content && message.content !== "[表情包]" ? message.content : "[表情包]";
      case 5:
        return message.content && message.content !== `[文件] ${message.fileName || ""}`
          ? message.content
          : message.fileName
            ? `[文件] ${message.fileName}`
            : "[文件]";
      default:
        return message.content || "暂无消息";
    }
  }

  function cloneMessageBase(msg) {
    return {
      clientMsgId: msg.clientMsgId,
      messageId: msg.messageId,
      conversationId: msg.conversationId,
      senderId: msg.senderId,
      senderUsername: msg.senderUsername,
      content: msg.content,
      seq: msg.seq,
      convType: msg.convType,
      cType: msg.cType,
      sendTime: msg.sendTime || Date.now(),
      videoId: msg.videoId || "",
      videoTime: msg.videoTime || null,
      mediaUrl: msg.mediaUrl || "",
      thumbUrl: msg.thumbUrl || "",
      fileId: msg.fileId || "",
      thumbFileId: msg.thumbFileId || "",
      fileName: msg.fileName || "",
      fileSize: msg.fileSize || 0,
      width: msg.width || 0,
      height: msg.height || 0,
      durationMs: msg.durationMs || null,
      stickerId: msg.stickerId || "",
      packId: msg.packId || "",
    };
  }

  function buildLatestMessage(message) {
    if (!message) return null;
    return {
      content: getMessagePreviewText(message),
      convType: message.convType,
      senderUsername: message.senderUsername,
      sendTime: message.sendTime || Date.now(),
      mediaUrl: message.mediaUrl || "",
      thumbUrl: message.thumbUrl || "",
      fileId: message.fileId || "",
      fileName: message.fileName || "",
      cType: message.cType,
    };
  }

  function formatOutgoingMediaContent(cType, fileName = "") {
    switch (Number(cType) || 0) {
      case 2:
        return "[图片]";
      case 3:
        return "[视频]";
      case 4:
        return "[表情包]";
      case 5:
        return fileName ? `[文件] ${fileName}` : "[文件]";
      default:
        return fileName || "";
    }
  }

  function mergeConversationMessages(existing = [], incoming = []) {
    const merged = [];
    const seen = new Set();
    for (const item of [...existing, ...incoming]) {
      if (!item) continue;
      const key = messageDedupKey(item);
      if (seen.has(key)) continue;
      seen.add(key);
      merged.push({ ...item });
    }
    merged.sort((a, b) => {
      const aSeq = Number(a.seq) || 0;
      const bSeq = Number(b.seq) || 0;
      if (aSeq > 0 && bSeq > 0 && aSeq !== bSeq) return aSeq - bSeq;
      const aTime = Number(a.sendTime) || 0;
      const bTime = Number(b.sendTime) || 0;
      if (aTime !== bTime) return aTime - bTime;
      return (a.clientMsgId || a.messageId || "").localeCompare(b.clientMsgId || b.messageId || "");
    });
    return merged;
  }

  function sendReadAck(conversationId, lastReadSeq, senderId) {
    if (!conversationId || !lastReadSeq || !senderId) return;
    sendFrame("msg_read_ack", "readAck", {
      conversationId,
      lastReadSeq,
      senderId,
    });
  }

  function scrollToBottom() {
    nextTick(() => {
      if (messageList.value) messageList.value.scrollTop = messageList.value.scrollHeight;
    });
  }

  function prunePendingLocalMessages() {
    const threshold = Date.now() - 15000;
    while (pendingLocalMessages.length && pendingLocalMessages[0].sendTime < threshold) {
      pendingLocalMessages.shift();
    }
  }

  function trackPendingLocalMessage(entry) {
    prunePendingLocalMessages();
    pendingLocalMessages.push(entry);
  }

  function removePendingLocalMessage(clientMsgId) {
    const index = pendingLocalMessages.findIndex((item) => item.clientMsgId === clientMsgId);
    if (index >= 0) pendingLocalMessages.splice(index, 1);
  }

  function isDuplicateLocalEcho(msg) {
    prunePendingLocalMessages();
    if (msg?.clientMsgId) {
      const clientIndex = pendingLocalMessages.findIndex((item) => item.clientMsgId === msg.clientMsgId);
      if (clientIndex >= 0) {
        pendingLocalMessages.splice(clientIndex, 1);
        return true;
      }
    }
    if (!msg?.sendId || msg.sendId !== currentUser.userId) return false;
    const index = pendingLocalMessages.findIndex((item) => {
      if (item.conversationId !== msg.conversationId) return false;
      if (item.sendId !== msg.sendId) return false;
      if (item.content !== msg.content) return false;
      if ((item.cType || 0) !== (msg.cType || 0)) return false;
      return Math.abs((msg.sendTime || Date.now()) - item.sendTime) < 15000;
    });
    if (index < 0) return false;
    pendingLocalMessages.splice(index, 1);
    return true;
  }

  function handleIncomingMessage(msg) {
    if (!msg || isDuplicateLocalEcho(msg)) return;
    const isActiveConversation = activeConversation.value?.conversationId === msg.conversationId;
    const targetId = Number(msg.convType) === 2
      ? (msg.recvId || msg.conversationId)
      : (msg.sendId === currentUser.userId ? msg.recvId : msg.sendId);
    upsertConversationPreview(
      msg.conversationId,
      {
        latestMessage: buildLatestMessage(msg),
        unread: isActiveConversation ? 0 : undefined,
        unreadDelta: isActiveConversation ? 0 : 1,
        convType: msg.convType,
        targetId,
        displayName:
          activeConversation.value?.conversationId === msg.conversationId
            ? activeConversation.value.displayName
            : Number(msg.convType) === 1
              ? (msg.senderUsername || "好友私聊")
              : "群聊",
      },
      true,
    );
    if (isActiveConversation) {
      messages.value.push(
        cloneMessageBase({
          clientMsgId: msg.clientMsgId,
          messageId: msg.messageId,
          conversationId: msg.conversationId,
          senderId: msg.sendId,
          senderUsername: msg.senderUsername,
          content: msg.content,
          seq: msg.seq,
          convType: msg.convType,
          cType: msg.cType,
          sendTime: msg.sendTime || Date.now(),
          videoTime: msg.videoTime || null,
          mediaUrl: msg.mediaUrl,
          thumbUrl: msg.thumbUrl,
          fileId: msg.fileId,
          thumbFileId: msg.thumbFileId,
          fileName: msg.fileName,
          fileSize: msg.fileSize,
          width: msg.width,
          height: msg.height,
          durationMs: msg.durationMs,
          stickerId: msg.stickerId,
          packId: msg.packId,
        }),
      );
      setConversationMessagesCache(msg.conversationId, messages.value);
      scrollToBottom();
    }
  }

  function handleWsFrame(frame) {
    if (frame.op === "msg") handleIncomingMessage(frame.payload);
    if (frame.op === "msg_ack") {
      const ack = frame.payload;
      if (ack.clientMsgId && ack.status === "failed") removePendingLocalMessage(ack.clientMsgId);
      if (ack.status === "failed") showMessage(ack.extra || "消息发送失败");
    }
    if (frame.op === "msg_read_notify") {
      const event = frame.payload || {};
      const conversationId = event.conversationId;
      const senderId = event.senderId;
      if (!conversationId || !senderId || senderId !== currentUser.userId) return;

      const existing = getConversationReadState(conversationId);
      const nextReaders = (existing.readers || []).filter((item) => item.userId !== event.userId);
      nextReaders.unshift({
        userId: event.userId,
        avatar: event.avatar || "",
        lastReadSeq: Number(event.lastReadSeq) || 0,
        convType: Number(event.convType) || 0,
        senderId: event.senderId,
      });

      readReceipts.value = {
        ...readReceipts.value,
        [conversationId]: {
          lastReadSeq: Math.max(Number(existing.lastReadSeq) || 0, Number(event.lastReadSeq) || 0),
          readers: nextReaders.slice(0, 4),
        },
      };

      if (activeConversation.value?.conversationId === conversationId) {
        activeReadState.value = readReceipts.value[conversationId];
      }
    }
  }

  async function loadConversations() {
    if (!token.value) return false;
    const data = await getConversations(token.value).catch((err) => {
      showMessage(err.message);
      return null;
    });
    if (!data) return false;
    conversations.value = (Array.isArray(data) ? data : []).map((item) => ({
      conversationId: item.conversationId,
      targetId: item.targetId || item.conversationId,
      displayName: item.displayName || "会话",
      avatar: item.avatar || item.peerUser?.avatar || item.room?.avatar || "",
      unread: Math.max(0, Number(item.unread) || 0),
      latestMessage: item.latestMessage,
      convType: Number(item.convType) || Number(item.latestMessage?.convType) || 2,
      peerUser: item.peerUser || null,
      room: item.room || null,
      isMuted: Boolean(item.isMuted),
    }));
    if (activeConversation.value?.conversationId) {
      const current = conversations.value.find((item) => item.conversationId === activeConversation.value.conversationId);
      if (current) activeConversation.value = current;
    }
    return true;
  }

  async function sendTextPayload(content) {
    if (!content || !activeConversation.value) return false;
    const clientMsgId = crypto.randomUUID();
    const ok = sendFrame("msg", "messageReq", {
      clientMsgId,
      recvId: activeConversation.value.targetId || activeConversation.value.conversationId,
      convType: activeConversation.value.convType || 2,
      cType: 1,
      content,
      videoTime: 0,
      hasVideoTime: false,
    });
    if (!ok) return false;
    const localMessage = cloneMessageBase({
      clientMsgId,
      senderId: currentUser.userId,
      senderUsername: currentUser.username,
      content,
      sendTime: Date.now(),
      convType: activeConversation.value.convType,
      cType: 1,
      videoTime: null,
    });
    trackPendingLocalMessage({
      clientMsgId,
      conversationId: activeConversation.value.conversationId,
      sendId: currentUser.userId,
      content,
      cType: 1,
      sendTime: Date.now(),
    });
    messages.value.push(localMessage);
    setConversationMessagesCache(activeConversation.value.conversationId, messages.value);
    upsertConversationPreview(
      activeConversation.value.conversationId,
      {
        displayName: activeConversation.value.displayName,
        unread: 0,
        resetUnread: true,
        latestMessage: buildLatestMessage(localMessage),
        convType: activeConversation.value.convType,
      },
      true,
    );
    messageText.value = "";
    scrollToBottom();
    return true;
  }

  async function sendMediaPayload(file, cType) {
    if (!file || !activeConversation.value) return false;
    const caption = messageText.value.trim();

    let uploaded;
    try {
      uploaded = await uploadFile(token.value, file);
    } catch (err) {
      showMessage(err.message);
      return false;
    }
    const meta = {
      fileId: uploaded.fileId || "",
      mediaUrl: uploaded.url || "",
      fileName: uploaded.fileName || file.name || "",
      fileSize: uploaded.size || file.size || 0,
      thumbUrl: uploaded.url || "",
      thumbFileId: "",
      width: 0,
      height: 0,
      durationMs: null,
      stickerId: "",
      packId: "",
    };

    if (cType === 2 || cType === 4) {
      const size = await getImageDimensions(file).catch(() => ({ width: 0, height: 0 }));
      meta.width = size.width;
      meta.height = size.height;
    }
    if (cType === 3) {
      const videoMeta = await getVideoMetadata(file).catch(() => ({ width: 0, height: 0, durationMs: null }));
      meta.width = videoMeta.width;
      meta.height = videoMeta.height;
      meta.durationMs = videoMeta.durationMs;
    }

    const content = caption || formatOutgoingMediaContent(cType, meta.fileName || file.name || "");
    const clientMsgId = crypto.randomUUID();
    const ok = sendFrame("msg", "messageReq", {
      clientMsgId,
      recvId: activeConversation.value.targetId || activeConversation.value.conversationId,
      convType: activeConversation.value.convType || 2,
      cType,
      content,
      mediaUrl: meta.mediaUrl,
      thumbUrl: meta.thumbUrl,
      fileId: meta.fileId,
      thumbFileId: meta.thumbFileId,
      fileName: meta.fileName,
      fileSize: meta.fileSize,
      width: meta.width,
      height: meta.height,
      durationMs: meta.durationMs || 0,
      stickerId: meta.stickerId,
      packId: meta.packId,
      hasVideoTime: false,
    });
    if (!ok) return false;

    const localMessage = cloneMessageBase({
      clientMsgId,
      senderId: currentUser.userId,
      senderUsername: currentUser.username,
      content,
      sendTime: Date.now(),
      convType: activeConversation.value.convType,
      cType,
      mediaUrl: meta.mediaUrl,
      thumbUrl: meta.thumbUrl,
      fileId: meta.fileId,
      thumbFileId: meta.thumbFileId,
      fileName: meta.fileName,
      fileSize: meta.fileSize,
      width: meta.width,
      height: meta.height,
      durationMs: meta.durationMs,
      stickerId: meta.stickerId,
      packId: meta.packId,
    });
    trackPendingLocalMessage({
      clientMsgId,
      conversationId: activeConversation.value.conversationId,
      sendId: currentUser.userId,
      content,
      cType,
      sendTime: Date.now(),
    });
    messages.value.push(localMessage);
    setConversationMessagesCache(activeConversation.value.conversationId, messages.value);
    upsertConversationPreview(
      activeConversation.value.conversationId,
      {
        displayName: activeConversation.value.displayName,
        unread: 0,
        resetUnread: true,
        latestMessage: buildLatestMessage(localMessage),
        convType: activeConversation.value.convType,
      },
      true,
    );
    messageText.value = "";
    scrollToBottom();
    return true;
  }

  function sendStickerMessageImpl(sticker) {
    if (!sticker || !activeConversation.value) return false;
    const clientMsgId = crypto.randomUUID();
    const content = sticker.alt || "[表情包]";
    const ok = sendFrame("msg", "messageReq", {
      clientMsgId,
      recvId: activeConversation.value.targetId || activeConversation.value.conversationId,
      convType: activeConversation.value.convType || 2,
      cType: 4,
      content,
      mediaUrl: sticker.url || "",
      thumbUrl: sticker.url || "",
      stickerId: sticker.stickerId || "",
      packId: sticker.packId || "",
      width: sticker.width || 0,
      height: sticker.height || 0,
      hasVideoTime: false,
    });
    if (!ok) return false;

    const localMessage = cloneMessageBase({
      clientMsgId,
      senderId: currentUser.userId,
      senderUsername: currentUser.username,
      content,
      sendTime: Date.now(),
      convType: activeConversation.value.convType,
      cType: 4,
      mediaUrl: sticker.url || "",
      thumbUrl: sticker.url || "",
      fileName: sticker.alt || "",
      stickerId: sticker.stickerId || "",
      packId: sticker.packId || "",
      width: sticker.width || 0,
      height: sticker.height || 0,
    });
    trackPendingLocalMessage({
      clientMsgId,
      conversationId: activeConversation.value.conversationId,
      sendId: currentUser.userId,
      content,
      cType: 4,
      sendTime: Date.now(),
    });
    messages.value.push(localMessage);
    setConversationMessagesCache(activeConversation.value.conversationId, messages.value);
    upsertConversationPreview(
      activeConversation.value.conversationId,
      {
        displayName: activeConversation.value.displayName,
        unread: 0,
        resetUnread: true,
        latestMessage: buildLatestMessage(localMessage),
        convType: activeConversation.value.convType,
      },
      true,
    );
    scrollToBottom();
    return true;
  }

  function getImageDimensions(file) {
    return new Promise((resolve, reject) => {
      const url = URL.createObjectURL(file);
      const image = new Image();
      image.onload = () => {
        URL.revokeObjectURL(url);
        resolve({ width: image.naturalWidth || 0, height: image.naturalHeight || 0 });
      };
      image.onerror = (err) => {
        URL.revokeObjectURL(url);
        reject(err || new Error("图片读取失败"));
      };
      image.src = url;
    });
  }

  function getVideoMetadata(file) {
    return new Promise((resolve, reject) => {
      const url = URL.createObjectURL(file);
      const video = document.createElement("video");
      video.preload = "metadata";
      video.onloadedmetadata = () => {
        URL.revokeObjectURL(url);
        resolve({
          width: video.videoWidth || 0,
          height: video.videoHeight || 0,
          durationMs: Number.isFinite(video.duration) ? Math.round(video.duration * 1000) : null,
        });
      };
      video.onerror = (err) => {
        URL.revokeObjectURL(url);
        reject(err || new Error("视频读取失败"));
      };
      video.src = url;
    });
  }

  async function selectConversation(item) {
    if (!item?.conversationId) return;

    const loadSeq = ++conversationLoadSeq;
    const conversationId = item.conversationId;
    const currentReadState = getConversationReadState(conversationId);
    const current = upsertConversationPreview(item.conversationId, {
      displayName: item.displayName,
      convType: item.convType,
      targetId: item.targetId,
      peerUser: item.peerUser,
      room: item.room,
      avatar: item.avatar,
      isMuted: item.isMuted,
      unread: 0,
      resetUnread: true,
      latestMessage: item.latestMessage || null,
    });
    activeConversation.value = current || item;
    const cachedMessages = getConversationMessagesCache(conversationId);
    if (cachedMessages) {
      messages.value = cachedMessages;
      conversationLoading.value = false;
    } else {
      messages.value = [];
      conversationLoading.value = true;
    }
    try {
      const history = await getHistoryMessages(
        token.value,
        item.conversationId,
        0,
        30,
        item.convType,
      ).catch((err) => {
        showMessage(err.message);
        return { messages: [] };
      });
      if (loadSeq !== conversationLoadSeq || activeConversation.value?.conversationId !== conversationId) return;
      const mergedMessages = mergeConversationMessages(
        getConversationMessagesCache(conversationId) || [],
        history.messages || [],
      );
      messages.value = mergedMessages;
      setConversationMessagesCache(conversationId, mergedMessages);
      const lastSeq = mergedMessages.filter((msg) => Number(msg.seq) > 0).at(-1)?.seq || 0;
      const lastSender = mergedMessages.filter((msg) => Number(msg.seq) > 0).at(-1)?.senderId || "";
      if (lastSeq > (Number(currentReadState.lastReadSeq) || 0)) {
        sendReadAck(item.conversationId, lastSeq, lastSender);
      }
      scrollToBottom();
    } finally {
      if (loadSeq === conversationLoadSeq) conversationLoading.value = false;
    }
  }

  function openConversation(conversationId, convType, content = "暂无消息") {
    if (!conversationId) return;
    const item = upsertConversationPreview(
      conversationId,
      {
        targetId: conversationId,
        displayName: content,
        unread: 0,
        resetUnread: true,
        latestMessage: { content, convType },
        convType,
      },
      true,
    );
    selectConversation(item);
  }

  function openPrivateConversation(friend) {
    const targetId = friend?.friendUserId;
    const existingConversation = conversations.value.find(
      (item) => Number(item.convType) === 1 && item.targetId === targetId,
    );
    const conversationId = existingConversation?.conversationId || buildPrivateConversationId(currentUser.userId, targetId);
    if (!conversationId) return;
    const item = upsertConversationPreview(
      conversationId,
      {
        targetId,
        displayName: friend.displayName || friend.friendUsername || friend.username || "好友",
        avatar: friend.friendAvatar || "",
        peerUser: friend
          ? {
              userId: friend.friendUserId || "",
              userName: friend.friendUsername || "",
              nickName: friend.displayName || "",
              remark: friend.displayName || "",
              avatar: friend.friendAvatar || "",
            }
          : null,
        unread: 0,
        resetUnread: true,
        latestMessage: { content: friend.displayName || "好友私聊", convType: 1 },
        convType: 1,
      },
      true,
    );
    selectConversation(item);
  }

  function sendMessage() {
    const content = messageText.value.trim();
    return sendTextPayload(content);
  }

  function sendImageMessage(file) {
    return sendMediaPayload(file, 2);
  }

  function sendVideoMessage(file) {
    return sendMediaPayload(file, 3);
  }

  function sendFileMessage(file) {
    return sendMediaPayload(file, 5);
  }

  function sendStickerMessage(sticker) {
    return sendStickerMessageImpl(sticker);
  }

  function buildPrivateConversationId(leftUserId, rightUserId) {
    if (!leftUserId || !rightUserId) return "";
    return leftUserId > rightUserId
      ? `${leftUserId}_${rightUserId}`
      : `${rightUserId}_${leftUserId}`;
  }

  function resetConversationState() {
    conversations.value = [];
    activeConversation.value = null;
    messages.value = [];
    readReceipts.value = {};
    activeReadState.value = { lastReadSeq: 0, readers: [] };
    conversationLoading.value = false;
    clearConversationMessagesCache();
    messageText.value = "";
  }

  function setMessageListRef(el) {
    messageList.value = el;
  }

  watch(
    [() => activeConversation.value?.conversationId, () => messages.value.length],
    () => {
      scrollToBottom();
    },
    { flush: "post" },
  );

  watch(
    () => activeConversation.value?.conversationId,
    (conversationId) => {
      activeReadState.value = getConversationReadState(conversationId);
    },
    { immediate: true },
  );

  return {
    conversations,
    activeConversation,
    messages,
    activeReadState,
    conversationLoading,
    messageText,
    messageList,
    loadConversations,
    selectConversation,
    openConversation,
    openPrivateConversation,
    sendMessage,
    sendImageMessage,
    sendVideoMessage,
    sendFileMessage,
    sendStickerMessage,
    handleWsFrame,
    setMessageListRef,
    resetConversationState,
    clearConversationMessagesCache,
    scrollToBottom,
  };
}
