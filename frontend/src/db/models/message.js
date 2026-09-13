export const MESSAGE_STORE_NAME = "messages";

export const MESSAGE_KEY_PATH = [
    "conversationId",
    "seq",
];

export const MESSAGE_INDEXES = {
    CONVERSATION: "conversation",
    CONVERSATION_SEQ: "conversationSeq",
};

/** 将消息转换为 IndexedDB 记录，只保留消息字段和 attachmentId。 */
export const createMessageRecord = (message) => {
    const seq = Number(message?.seq);
    // 正常消息使用 conversationId + seq 作为 IndexedDB 主键。
    if (!message?.conversationId || !message?.messageId || !Number.isSafeInteger(seq) || seq <= 0) {
        return null;
    }

    return {
        messageId: message.messageId,
        conversationId: message.conversationId,
        senderId: message.senderId || "",
        clientMsgId: message.clientMsgId || "",
        seq,
        cType: Number(message.cType) || 1,
        content: message.content || "",

        // 弹幕相关接口兼容
        videoId: message.videoId || "",
        videoTime: message.videoTime ?? null,
        status: message.status || "sent",
        sendTime: Number(message.sendTime) || 0,
        attachmentId: message.attachmentId || "",
    };
};
