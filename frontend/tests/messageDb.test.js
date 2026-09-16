import "fake-indexeddb/auto";
import assert from "node:assert/strict";
import { test, before, after } from "node:test";

globalThis.window = globalThis;

const DB_NAME = "im-chat-cache";
const USER_ID = "test-user";
const CONVERSATION_ID = "test-conversation";
const SECOND_CONVERSATION_ID = "test-conversation-2";

const {
    clearConversations,
    queryConversationsByUserId,
    replaceConversations,
} = await import("../src/db/services/conversationService.js");
const {
    clearMessages,
    deleteMessagesByConversation,
    insertMessages,
    queryMessagesByCursor,
} = await import("../src/db/services/messageService.js");

const deleteDatabase = () => new Promise((resolve, reject) => {
    const request = indexedDB.deleteDatabase(DB_NAME);
    request.onsuccess = () => resolve();
    request.onerror = () => reject(request.error);
    request.onblocked = () => reject(new Error("IndexedDB 删除被阻塞"));
});

before(async () => {
    await deleteDatabase();
});

after(async () => {
    await clearMessages();
    await clearConversations();
    await deleteDatabase();
});

test("会话快照可以替换，消息与调用方确认的连续序号原子写入", async () => {
    await replaceConversations(USER_ID, [{
        conversationId: CONVERSATION_ID,
        convType: 1,
        displayName: "测试会话",
        lastMessage: { sendTime: 1000 },
    }]);

    const conversations = await queryConversationsByUserId(USER_ID);
    assert.equal(conversations.length, 1);
    assert.equal(conversations[0].conversationId, CONVERSATION_ID);

    const insertResult = await insertMessages([
        {
            conversationId: CONVERSATION_ID,
            messageId: "message-1",
            senderId: "sender",
            seq: 1,
            content: "一",
            status: 2,
            sendTime: 1000,
        },
        {
            conversationId: CONVERSATION_ID,
            messageId: "message-2",
            senderId: "sender",
            seq: 2,
            content: "二",
            sendTime: 2000,
        },
        {
            conversationId: CONVERSATION_ID,
            messageId: "message-3",
            senderId: "sender",
            seq: 3,
            content: "三",
            sendTime: 3000,
        },
    ], {
        userId: USER_ID,
        lastContinuousSeqByConversation: {
            [CONVERSATION_ID]: 3,
        },
    });

    assert.equal(
        insertResult.lastContinuousSeqByConversation[CONVERSATION_ID],
        3,
    );
    const updatedConversations = await queryConversationsByUserId(USER_ID);
    assert.equal(updatedConversations[0].lastContinuousSeq, 3);

    const latestPage = await queryMessagesByCursor({
        conversationId: CONVERSATION_ID,
        limit: 2,
    });
    assert.deepEqual(
        latestPage.messages.map((message) => message.seq),
        [2, 3],
    );
    assert.equal(latestPage.hasMore, true);
    assert.equal(latestPage.nextCursor, 2);

    const messageWithBusinessStatus = await queryMessagesByCursor({
        conversationId: CONVERSATION_ID,
        limit: 2,
    });
    assert.equal(messageWithBusinessStatus.messages[0].status, 1);

    const recalledMessage = await queryMessagesByCursor({
        conversationId: CONVERSATION_ID,
        cursor: 2,
        limit: 2,
    });
    assert.equal(recalledMessage.messages[0].status, 2);

    const olderPage = await queryMessagesByCursor({
        conversationId: CONVERSATION_ID,
        cursor: latestPage.nextCursor,
        limit: 2,
    });
    assert.deepEqual(
        olderPage.messages.map((message) => message.seq),
        [1],
    );
    assert.equal(olderPage.hasMore, false);
    assert.equal(olderPage.nextCursor, -1);
});

test("相同 conversationId + seq 的消息写入具有幂等覆盖语义", async () => {
    await replaceConversations(USER_ID, [{
        conversationId: SECOND_CONVERSATION_ID,
        convType: 1,
    }]);

    await insertMessages([{
        conversationId: SECOND_CONVERSATION_ID,
        messageId: "message-replaced",
        senderId: "sender",
        seq: 1,
        content: "覆盖后的消息",
        sendTime: 4000,
    }], {
        userId: USER_ID,
        lastContinuousSeqByConversation: {
            [SECOND_CONVERSATION_ID]: 1,
        },
    });

    const page = await queryMessagesByCursor({
        conversationId: SECOND_CONVERSATION_ID,
        limit: 10,
    });
    assert.equal(page.messages.length, 1);
    assert.equal(page.messages[0].messageId, "message-replaced");
    assert.equal(page.messages[0].content, "覆盖后的消息");

    await deleteMessagesByConversation(USER_ID, SECOND_CONVERSATION_ID);
    const emptyPage = await queryMessagesByCursor({
        conversationId: SECOND_CONVERSATION_ID,
    });
    assert.deepEqual(emptyPage.messages, []);
    assert.equal(emptyPage.hasMore, false);
    const conversations = await queryConversationsByUserId(USER_ID);
    const conversation = conversations.find((item) => item.conversationId === SECOND_CONVERSATION_ID);
    assert.equal(conversation.lastContinuousSeq, 0);
});
