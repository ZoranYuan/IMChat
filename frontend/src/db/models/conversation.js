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
        // 保留后端 /conversations 返回的字段，不在前端重新改名。
        ...conversation,
        userId,
    };
};
