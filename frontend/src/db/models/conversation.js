export const CONVERSATION_STORE_NAME =
    "conversations";

export const LEGACY_USER_CONVERSATION_STORE_NAME =
    "userConversations";

export const CONVERSATION_KEY_PATH = [
    "userId",
    "conversationId",
];

export const CONVERSATION_INDEXES = {
    USER_ID: "userId",
};

/** 将后端会话对象附加当前用户 ID，转换为 IndexedDB 会话记录。 */
export const createConversationRecord = (
    userId,
    conversation = {},
) => {
    if (!userId || !conversation.conversationId) return null;

    return {
        userId,
        conversationId: conversation.conversationId,
        convType: conversation.convType,
        targetId: conversation.targetId,
        displayName: conversation.displayName,
        avatar: conversation.avatar,
        lastMessage: conversation.lastMessage,
        peerUser: conversation.peerUser,
        room: conversation.room,
        // 本地消息同步边界，初始值为 0，之后由消息落库流程维护。
        lastContinuousSeq: Number(conversation.lastContinuousSeq) || 0,
    };
};
