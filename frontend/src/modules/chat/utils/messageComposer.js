export const createClientMessageId = () => (
  globalThis.crypto?.randomUUID?.()
  || `message-${Date.now()}-${Math.random().toString(16).slice(2)}`
);

export const createOutgoingMessage = (conversation, payload, currentUser) => {
  const sendTime = Date.now();
  return {
    messageId: "",
    conversationId: conversation.conversationId,
    clientMsgId: payload.clientMsgId,
    senderId: currentUser.userId,
    senderUsername: currentUser.nickName || currentUser.username || "我",
    seq: 0,
    content: payload.content || "",
    cType: payload.cType,
    convType: payload.convType,
    fileId: payload.fileId || "",
    fileName: payload.fileName || "",
    fileSize: payload.fileSize || 0,
    mediaUrl: payload.mediaUrl || "",
    thumbUrl: payload.thumbUrl || "",
    width: payload.width || 0,
    height: payload.height || 0,
    durationMs: payload.durationMs || 0,
    sendTime,
    status: "sending",
  };
};

export const createConversationPreview = (conversation, message, currentUser) => ({
  messageId: message.messageId || "",
  conversationId: conversation.conversationId,
  senderId: currentUser.userId,
  seq: Number(message.seq) || 0,
  convType: conversation.convType,
  cType: message.cType,
  content: message.content || message.fileName || "",
  sendTime: message.sendTime || Date.now(),
});
