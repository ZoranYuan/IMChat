import assert from "node:assert/strict";
import test from "node:test";
import "fake-indexeddb/auto";

globalThis.window = globalThis;

const { deleteMessages, readMessages, writeMessages } = await import("../src/services/messageDb.js");

test("stores, isolates, orders, and deletes conversation messages", async () => {
  const userId = `user-${Date.now()}`;
  const otherUserId = `${userId}-other`;
  const conversationId = "conversation-1";

  await writeMessages(userId, conversationId, [
    {
      id: "message-2",
      senderId: "user-b",
      seq: 2,
      convType: 1,
      cType: 1,
      content: "second",
      sendTime: 2000,
    },
    {
      id: "message-1",
      senderId: userId,
      seq: 1,
      convType: 1,
      cType: 1,
      content: "first",
      sendTime: 1000,
    },
  ]);
  await writeMessages(otherUserId, conversationId, [{
    id: "other-message",
    senderId: otherUserId,
    seq: 1,
    convType: 1,
    cType: 1,
    content: "private",
    sendTime: 1000,
  }]);

  const messages = await readMessages(userId, conversationId);
  assert.deepEqual(messages.map((message) => message.messageId), ["message-1", "message-2"]);
  assert.deepEqual(
    messages.map(({ senderId, seq, convType, cType, content, sendTime }) => ({
      senderId, seq, convType, cType, content, sendTime,
    })),
    [
      { senderId: userId, seq: 1, convType: 1, cType: 1, content: "first", sendTime: 1000 },
      { senderId: "user-b", seq: 2, convType: 1, cType: 1, content: "second", sendTime: 2000 },
    ],
  );

  await deleteMessages(userId, conversationId);
  assert.deepEqual(await readMessages(userId, conversationId), []);
  assert.equal((await readMessages(otherUserId, conversationId)).length, 1);

  await deleteMessages(otherUserId, conversationId);
});
