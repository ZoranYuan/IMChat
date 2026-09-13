import {
    MESSAGE_INDEXES,
    MESSAGE_STORE_NAME,
    createMessageRecord,
} from "../models/message.js";

import { CONVERSATION_STORE_NAME } from "../models/conversation.js";

import {
    closeIndexedDb,
    openIndexedDb,
} from "./indexedDb.js";

const MAX_LIMIT = 100;

/** 推进指定会话的本地连续消息序号。 */
export const advanceLastContinuousSeq = async (
    userId,
    conversationId,
) => {
    if (!userId || !conversationId) return 0;

    const db = await openIndexedDb();

    return new Promise((resolve, reject) => {
        const transaction = db.transaction(
            [
                MESSAGE_STORE_NAME,
                CONVERSATION_STORE_NAME,
            ],
            "readwrite",
        );

        const messageStore = transaction.objectStore(
            MESSAGE_STORE_NAME,
        );

        const conversationStore = transaction.objectStore(
            CONVERSATION_STORE_NAME,
        );

        let resultSeq = 0;

        transaction.oncomplete = () => {
            closeIndexedDb(db);
            resolve(resultSeq);
        };

        transaction.onerror = () => {
            closeIndexedDb(db);
            reject(transaction.error);
        };

        transaction.onabort = () => {
            closeIndexedDb(db);
            reject(
                transaction.error ||
                new Error("更新 lastContinuousSeq 失败"),
            );
        };

        const conversationRequest = conversationStore.get([
            userId,
            conversationId,
        ]);

        conversationRequest.onsuccess = () => {
            const conversation = conversationRequest.result;

            if (!conversation) {
                return;
            }

            const currentSeq = Math.max(
                Number(conversation.lastContinuousSeq) || 0,
                0,
            );

            resultSeq = currentSeq;

            let expectedSeq = currentSeq + 1;

            const finish = () => {
                const nextSeq = expectedSeq - 1;

                if (nextSeq > currentSeq) {
                    conversation.lastContinuousSeq = nextSeq;
                    conversationStore.put(conversation);
                }

                resultSeq = nextSeq;
            };

            const range = IDBKeyRange.bound(
                [conversationId, expectedSeq],
                [conversationId, Number.MAX_SAFE_INTEGER],
            );

            const cursorRequest = messageStore
                .index(MESSAGE_INDEXES.CONVERSATION_SEQ)
                .openCursor(range, "next");

            cursorRequest.onsuccess = () => {
                const cursor = cursorRequest.result;

                if (!cursor) {
                    finish();
                    return;
                }

                const seq = Number(cursor.value.seq);

                // seq 大于 expectedSeq，表示中间存在断层。
                if (!Number.isSafeInteger(seq) || seq > expectedSeq) {
                    finish();
                    return;
                }

                if (seq < expectedSeq) {
                    cursor.continue();
                    return;
                }

                // 找到连续的下一条消息，继续检查。
                expectedSeq = seq + 1;
                cursor.continue();
            };
        };
    });
};


/** 在一个事务中批量写入消息表。 */
const insertMessageRecords = async (records) => {
    const db = await openIndexedDb();

    return new Promise((resolve, reject) => {
        const transaction = db.transaction(
            MESSAGE_STORE_NAME,
            "readwrite",
        );

        const store = transaction.objectStore(
            MESSAGE_STORE_NAME,
        );

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

        for (const record of records) {
            // 相同 conversationId、seq 会覆盖，天然幂等。
            store.put(record);
        }
    });
};

/** 批量写入消息，并维护对应会话的 lastContinuousSeq。 */
export const insertMessages = async (messages, { userId } = {}) => {
    if (!messages?.length) {
        return {
            lastContinuousSeqByConversation: {},
        };
    }

    if (!userId) {
        throw new Error("insertMessages requires userId");
    }

    const records = messages
        .map((message) => createMessageRecord(message))
        .filter(Boolean);

    if (!records.length) {
        return {
            lastContinuousSeqByConversation: {},
        };
    }

    // 先完成消息表写入，再维护会话表，避免两个事务互相读取未提交数据。
    await insertMessageRecords(records);

    const conversationIds = [
        ...new Set(
            records.map((record) => record.conversationId),
        ),
    ];

    const results = await Promise.all(
        conversationIds.map(async (conversationId) => ({
            conversationId,
            lastContinuousSeq: await advanceLastContinuousSeq(
                userId,
                conversationId,
            ),
        })),
    );

    return {
        lastContinuousSeqByConversation: Object.fromEntries(
            results.map((result) => [
                result.conversationId,
                result.lastContinuousSeq,
            ]),
        ),
    };
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
