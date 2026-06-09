import { nextTick, ref, watch } from "vue";
import { getHistoryMessages, getOfflineMessages } from "../api";

export function useConversation({ token, currentUser, showMessage, sendFrame, onRoomConversationSelected, getWatchVideoTime }) {
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
      displayName: patch.displayName,
      avatar: patch.avatar,
      unread: nextUnread,
      latestMessage: patch.latestMessage ?? existing?.latestMessage ?? null,
      convType: patch.convType ?? existing?.convType ?? patch.latestMessage?.convType ?? 2,
    };

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
    upsertConversationPreview(
      msg.conversationId,
      {
        latestMessage: {
          content: msg.content,
          convType: msg.convType,
          senderUsername: msg.senderUsername,
          sendTime: msg.sendTime || Date.now(),
        },
        unread: isActiveConversation ? 0 : undefined,
        unreadDelta: isActiveConversation ? 0 : 1,
        convType: msg.convType,
      },
      true,
    );
    if (isActiveConversation) {
      messages.value.push({
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
      });
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

  async function loadOffline() {
    if (!token.value) return false;
    const data = await getOfflineMessages(token.value).catch((err) => {
      showMessage(err.message);
      return null;
    });
    if (!data) return false;
    conversations.value = (Array.isArray(data) ? data : []).map((item) => ({
      conversationId: item.conversationId,
      displayName: item.displayName || item.latestMessage?.displayName || "会话",
      avatar: item.avatar || item.latestMessage?.avatar || "",
      unread: Math.max(0, Number(item.unread) || 0),
      latestMessage: item.latestMessage,
      convType: item.latestMessage?.convType || 2,
    }));
    if (activeConversation.value?.conversationId) {
      const current = conversations.value.find((item) => item.conversationId === activeConversation.value.conversationId);
      if (current) activeConversation.value = current;
    }
    return true;
  }

  async function selectConversation(item) {
    if (!item?.conversationId) return;

    // 离开当前会话时，ack 最后读到的那条消息
    const prevConversation = activeConversation.value;
    if (prevConversation && prevConversation.conversationId !== item.conversationId) {
      const prevMsgs = getConversationMessagesCache(prevConversation.conversationId) || messages.value;
      const lastMsg = prevMsgs.filter((msg) => Number(msg.seq) > 0).at(-1);
      if (lastMsg) {
        sendReadAck(prevConversation.conversationId, lastMsg.seq, lastMsg.senderId || "");
      }
    }

    const loadSeq = ++conversationLoadSeq;
    const requestedConversationId = item.conversationId;
    const current = upsertConversationPreview(item.conversationId, {
      displayName: item.displayName,
      convType: item.convType,
      unread: 0,
      resetUnread: true,
      latestMessage: item.latestMessage || null,
    });
    activeConversation.value = current || item;
    const cachedMessages = getConversationMessagesCache(requestedConversationId);
    if (cachedMessages) {
      messages.value = cachedMessages;
      conversationLoading.value = false;
    } else {
      messages.value = [];
      conversationLoading.value = true;
    }
    try {
      const history = await getHistoryMessages(token.value, item.conversationId).catch((err) => {
        showMessage(err.message);
        return { messages: [] };
      });
      if (loadSeq !== conversationLoadSeq || activeConversation.value?.conversationId !== requestedConversationId) return;
      const mergedMessages = mergeConversationMessages(
        getConversationMessagesCache(requestedConversationId) || [],
        history.messages || [],
      );
      messages.value = mergedMessages;
      setConversationMessagesCache(requestedConversationId, mergedMessages);
      const lastSeq = mergedMessages.filter((msg) => Number(msg.seq) > 0).at(-1)?.seq || 0;
      const lastSender = mergedMessages.filter((msg) => Number(msg.seq) > 0).at(-1)?.senderId || "";
      sendReadAck(item.conversationId, lastSeq, lastSender);
      scrollToBottom();
      if (current?.convType === 2 && typeof onRoomConversationSelected === "function") {
        onRoomConversationSelected(current);
      }
    } finally {
      if (loadSeq === conversationLoadSeq) conversationLoading.value = false;
    }
  }

  function openConversation(conversationId, convType, content = "暂无消息") {
    if (!conversationId) return;
    const item = upsertConversationPreview(
      conversationId,
      {
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
    const conversationId = friend.friendUserId || friend.toUserId;
    if (!conversationId) return;
    const item = upsertConversationPreview(
      conversationId,
      {
        displayName: friend.displayName || friend.friendUsername || friend.username || "好友",
        unread: 0,
        resetUnread: true,
        latestMessage: { content: friend.displayName || "好友私聊", convType: 1 },
        convType: 1,
      },
      true,
    );
    selectConversation(item);
  }

  function sendMessage(options = {}) {
    const content = messageText.value.trim();
    if (!content || !activeConversation.value) return;
    const clientMsgId = crypto.randomUUID();
    const videoTime = options.videoTime ?? (typeof getWatchVideoTime === "function" ? getWatchVideoTime() : 0);
    const ok = sendFrame("msg", "messageReq", {
      clientMsgId,
      recvId: activeConversation.value.conversationId,
      convType: activeConversation.value.convType || 2,
      cType: 1,
      content,
      videoTime,
      hasVideoTime: Boolean(options.withVideoContext),
    });
    if (!ok) return;
    trackPendingLocalMessage({
      clientMsgId,
      conversationId: activeConversation.value.conversationId,
      sendId: currentUser.userId,
      content,
      cType: 1,
      sendTime: Date.now(),
    });
    messages.value.push({
      clientMsgId,
      senderId: currentUser.userId,
      senderUsername: currentUser.username,
      content,
      sendTime: Date.now(),
      convType: activeConversation.value.convType,
      videoId: "",
      videoTime: options.withVideoContext ? videoTime : null,
    });
    setConversationMessagesCache(activeConversation.value.conversationId, messages.value);
    upsertConversationPreview(
      activeConversation.value.conversationId,
      {
        displayName: activeConversation.value.displayName,
        unread: 0,
        resetUnread: true,
        latestMessage: {
          content,
          convType: activeConversation.value.convType,
          senderUsername: currentUser.username,
          sendTime: Date.now(),
        },
        convType: activeConversation.value.convType,
      },
      true,
    );
    messageText.value = "";
    scrollToBottom();
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
    loadOffline,
    selectConversation,
    openConversation,
    openPrivateConversation,
    sendMessage,
    handleWsFrame,
    setMessageListRef,
    resetConversationState,
    clearConversationMessagesCache,
    scrollToBottom,
  };
}
