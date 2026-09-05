export const findConversation = (conversations, conversationId) => (
  (conversations || []).find((item) => item.conversationId === conversationId) || null
);

export const sortConversations = (conversations = []) => [...conversations].sort((left, right) => {
  const rightTime = Number(right.lastMessage?.sendTime) || 0;
  const leftTime = Number(left.lastMessage?.sendTime) || 0;
  if (rightTime !== leftTime) return rightTime - leftTime;
  return (Number(right.latestSeq) || 0) - (Number(left.latestSeq) || 0);
});

export const applyIncomingMessageToConversation = (
  conversation,
  message,
  { active = false } = {},
) => {
  if (!conversation || !message) return false;

  const seq = Number(message.seq) || 0;
  const currentLatestSeq = Number(conversation.latestSeq) || 0;
  const currentLastMessageSeq = Number(conversation.lastMessage?.seq) || 0;
  const isNew = seq > currentLatestSeq;

  conversation.latestSeq = Math.max(currentLatestSeq, seq);
  if (!conversation.lastMessage || seq > currentLastMessageSeq) {
    conversation.lastMessage = { ...message };
  }

  if (active) {
    conversation.unread = 0;
  } else if (isNew) {
    conversation.unread = (Number(conversation.unread) || 0) + 1;
  }

  return isNew;
};

export const applySyncedMessagesToConversation = (
  conversation,
  messages,
  { active = false } = {},
) => {
  if (!conversation || !messages?.length) return;

  const latest = messages.reduce((current, message) => (
    !current || Number(message.seq) > Number(current.seq) ? message : current
  ), null);
  if (latest) applyIncomingMessageToConversation(conversation, latest, { active });

  if (active) {
    conversation.unread = 0;
  } else {
    conversation.unread = Math.max(
      (Number(conversation.latestSeq) || 0) - (Number(conversation.lastReadSeq) || 0),
      0,
    );
  }
};

export const createDirectConversation = (currentUserId, contact) => {
  const conversationId = [currentUserId, contact.id].sort().reverse().join("_");
  return {
    conversationId,
    targetId: contact.id,
    convType: 1,
    displayName: contact.name,
    unread: 0,
    latestSeq: 0,
    lastReadSeq: 0,
    lastMessage: null,
    avatar: contact.avatar || "",
    peerUser: {
      userId: contact.id,
      userName: contact.username || "",
      nickName: contact.name,
      remark: "",
      avatar: contact.avatar || "",
    },
    room: null,
    isMuted: false,
  };
};
