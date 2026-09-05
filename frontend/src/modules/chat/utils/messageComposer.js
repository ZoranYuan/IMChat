export const createClientMessageId = () => (
  globalThis.crypto?.randomUUID?.()
  || `message-${Date.now()}-${Math.random().toString(16).slice(2)}`
);

export const getImageDimensions = (file) => new Promise((resolve, reject) => {
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

export const getVideoMetadata = (file) => new Promise((resolve, reject) => {
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
