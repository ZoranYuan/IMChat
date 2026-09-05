import {
    MESSAGE_INDEXES,
    MESSAGE_STORE_NAME,
    createMessageRecord,
} from "../models/message.js";

import {
    closeIndexedDb,
    openIndexedDb,
} from "./indexedDb.js";

const MAX_LIMIT = 100;

/** 在一个 IndexedDB 事务中批量写入已确认消息。 */
export const insertMessages = async (messages) => {
    if (!messages?.length) {
        return;
    }

    const db = await openIndexedDb();

    return new Promise((resolve, reject) => {
        const transaction = db.transaction(
            MESSAGE_STORE_NAME,
            "readwrite",
        );

        const store = transaction.objectStore(
            MESSAGE_STORE_NAME,
        );

        for (const message of messages) {
            const record = createMessageRecord(message);

            if (!record) continue;

            // 相同 conversationId、seq 会覆盖
            store.put(record);
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
            reject(
                transaction.error ||
                new Error("消息批量写入事务被取消"),
            );
        };
    });
};

/** 按会话和 cursor 倒序读取本地历史消息，并返回本地分页游标。 */
export const queryMessagesByCursor = async ({
    conversationId,
    cursor = 0,
    limit = 30,
}) => {
    if (!conversationId) {
        return {
            messages: [],
            hasMore: false,
            nextCursor: -1,
        };
    }

    if (Number(cursor) === -1) {
        return {
            messages: [],
            hasMore: false,
            nextCursor: -1,
        };
    }

    const currentCursor = Math.max(Number(cursor) || 0, 0);

    const pageSize = Math.min(
        Math.max(Number(limit) || 30, 1),
        MAX_LIMIT,
    );

    const db = await openIndexedDb();

    return new Promise((resolve, reject) => {
        const transaction = db.transaction(
            MESSAGE_STORE_NAME,
            "readonly",
        );

        const store = transaction.objectStore(
            MESSAGE_STORE_NAME,
        );

        const index = store.index(
            MESSAGE_INDEXES.CONVERSATION_SEQ,
        );

        const lowerKey = [
            conversationId,
            0,
        ];

        const upperKey =
            currentCursor > 0
                ? [conversationId, currentCursor]
                : [
                    conversationId,
                    Number.MAX_SAFE_INTEGER,
                ];

        const range = IDBKeyRange.bound(
            lowerKey,
            upperKey,
            false,
            currentCursor > 0,
        );

        const records = [];
        const request = index.openCursor(
            range,
            "prev",
        );

        request.onsuccess = () => {
            const result = request.result;

            if (!result) {
                return;
            }

            records.push(result.value);

            // 多取一条，判断本地是否还有更早消息
            if (records.length <= pageSize) {
                result.continue();
            }
        };

        request.onerror = () => {
            reject(request.error);
        };

        transaction.oncomplete = () => {
            closeIndexedDb(db);

            const hasMore = records.length > pageSize;

            const pageRecords = records
                .slice(0, pageSize)
                .reverse();

            resolve({
                messages: pageRecords,
                hasMore,
                nextCursor: hasMore ? Number(pageRecords[0]?.seq) || -1 : -1,
            });
        };

        transaction.onerror = () => {
            closeIndexedDb(db);
            reject(transaction.error);
        };
    });
};

/** 删除指定会话的全部本地消息。 */
export const deleteMessagesByConversation = async (conversationId) => {
    if (!conversationId) return;

    const db = await openIndexedDb();
    return new Promise((resolve, reject) => {
        const transaction = db.transaction(MESSAGE_STORE_NAME, "readwrite");
        const index = transaction.objectStore(MESSAGE_STORE_NAME).index(MESSAGE_INDEXES.CONVERSATION);
        const request = index.openCursor(IDBKeyRange.only(conversationId));

        request.onsuccess = () => {
            const cursor = request.result;
            if (!cursor) return;
            cursor.delete();
            cursor.continue();
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
            reject(transaction.error || new Error("删除消息事务被取消"));
        };
    });
};

/** 清空本地消息表，通常在退出登录时调用。 */
export const clearMessages = async () => {
    const db = await openIndexedDb();
    return new Promise((resolve, reject) => {
        const transaction = db.transaction(MESSAGE_STORE_NAME, "readwrite");
        transaction.objectStore(MESSAGE_STORE_NAME).clear();
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
            reject(transaction.error || new Error("清理消息事务被取消"));
        };
    });
};
