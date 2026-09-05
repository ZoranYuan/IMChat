import {
    CONVERSATION_INDEXES,
    CONVERSATION_STORE_NAME,
    createConversationRecord,
} from "../models/conversation.js";
import {
    closeIndexedDb,
    openIndexedDb,
} from "./indexedDb.js";

/** 合并已有本地会话和最新会话快照，保留未被服务端覆盖的本地字段。 */
const mergeConversationRecord = (oldRecord, nextRecord) => ({
    ...oldRecord,
    ...nextRecord,
});

/** 在一个 IndexedDB 事务中批量新增或更新用户会话。 */
export const insertConversations = async (userId, conversations) => {
    if (!userId || !conversations?.length) return;

    const db = await openIndexedDb();
    return new Promise((resolve, reject) => {
        const transaction = db.transaction(CONVERSATION_STORE_NAME, "readwrite");
        const store = transaction.objectStore(CONVERSATION_STORE_NAME);

        for (const conversation of conversations) {
            const record = createConversationRecord(userId, conversation);
            if (!record) continue;

            const readRequest = store.get([userId, record.conversationId]);
            readRequest.onsuccess = () => {
                store.put(mergeConversationRecord(readRequest.result, record));
            };
            readRequest.onerror = () => reject(readRequest.error);
        }

        transaction.oncomplete = () => {
            closeIndexedDb(db);
            resolve();
        };
        transaction.onerror = () => {
            closeIndexedDb(db);
            reject(transaction.error);
        };
        transaction.onabort = () => {
            closeIndexedDb(db);
            reject(transaction.error || new Error("会话批量写入事务被取消"));
        };
    });
};

/** 用服务端会话快照替换指定用户的本地会话列表。 */
export const replaceConversations = async (userId, conversations) => {
    if (!userId) return;

    const db = await openIndexedDb();
    return new Promise((resolve, reject) => {
        const transaction = db.transaction(CONVERSATION_STORE_NAME, "readwrite");
        const store = transaction.objectStore(CONVERSATION_STORE_NAME);
        const index = store.index(CONVERSATION_INDEXES.USER_ID);
        const request = index.getAllKeys(IDBKeyRange.only(userId));

        request.onsuccess = () => {
            for (const key of request.result || []) store.delete(key);
            for (const conversation of conversations || []) {
                const record = createConversationRecord(userId, conversation);
                if (record) store.put(record);
            }
        };
        request.onerror = () => reject(request.error);
        transaction.oncomplete = () => {
            closeIndexedDb(db);
            resolve();
        };
        transaction.onerror = () => {
            closeIndexedDb(db);
            reject(transaction.error);
        };
        transaction.onabort = () => {
            closeIndexedDb(db);
            reject(transaction.error || new Error("会话快照替换事务被取消"));
        };
    });
};

/** 按用户 ID 查询本地会话，并按最后一条消息时间排序。 */
export const queryConversationsByUserId = async (userId) => {
    if (!userId) return [];

    const db = await openIndexedDb();
    return new Promise((resolve, reject) => {
        const transaction = db.transaction(CONVERSATION_STORE_NAME, "readonly");
        const index = transaction.objectStore(CONVERSATION_STORE_NAME).index(CONVERSATION_INDEXES.USER_ID);
        const request = index.getAll(IDBKeyRange.only(userId));

        request.onsuccess = () => {
            const result = request.result || [];
            result.sort((left, right) => (
                Number(right.lastMessage?.sendTime || 0) -
                Number(left.lastMessage?.sendTime || 0)
            ));
            resolve(result);
        };
        request.onerror = () => reject(request.error);
        transaction.oncomplete = () => closeIndexedDb(db);
        transaction.onerror = () => {
            closeIndexedDb(db);
            reject(transaction.error);
        };
    });
};

/** 更新本地会话的已读序号，并重新计算本地未读数。 */
export const updateConversationLastReadSeq = async ({ userId, conversationId, lastReadSeq }) => {
    if (!userId || !conversationId || Number(lastReadSeq) <= 0) return;

    const db = await openIndexedDb();
    return new Promise((resolve, reject) => {
        const transaction = db.transaction(CONVERSATION_STORE_NAME, "readwrite");
        const store = transaction.objectStore(CONVERSATION_STORE_NAME);
        const key = [userId, conversationId];
        const readRequest = store.get(key);

        readRequest.onsuccess = () => {
            const oldRecord = readRequest.result || {
                userId,
                conversationId,
            };
            const nextLastReadSeq = Math.max(
                Number(oldRecord.lastReadSeq) || 0,
                Number(lastReadSeq),
            );
            store.put({
                ...oldRecord,
                lastReadSeq: nextLastReadSeq,
                unread: Math.max(
                    (Number(oldRecord.latestSeq) || 0) - nextLastReadSeq,
                    0,
                ),
            });
        };
        readRequest.onerror = () => reject(readRequest.error);
        transaction.oncomplete = () => {
            closeIndexedDb(db);
            resolve();
        };
        transaction.onerror = () => {
            closeIndexedDb(db);
            reject(transaction.error);
        };
    });
};

/** 清空本地会话表，通常在退出登录时调用。 */
export const clearConversations = async () => {
    const db = await openIndexedDb();
    return new Promise((resolve, reject) => {
        const transaction = db.transaction(CONVERSATION_STORE_NAME, "readwrite");
        transaction.objectStore(CONVERSATION_STORE_NAME).clear();
        transaction.oncomplete = () => {
            closeIndexedDb(db);
            resolve();
        };
        transaction.onerror = () => {
            closeIndexedDb(db);
            reject(transaction.error);
        };
    });
};
