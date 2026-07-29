const DB_NAME = "im-chat-cache";
const DB_VERSION = 1;
const STORE_NAME = "messages";

const openDb = () => new Promise((resolve, reject) => {
  if (typeof window === "undefined" || !("indexedDB" in window)) {
    reject(new Error("IndexedDB is unavailable"));
    return;
  }
  const request = window.indexedDB.open(DB_NAME, DB_VERSION);
  request.onupgradeneeded = () => {
    const db = request.result;
    const store = db.objectStoreNames.contains(STORE_NAME)
      ? request.transaction.objectStore(STORE_NAME)
      : db.createObjectStore(STORE_NAME, { keyPath: "cacheId" });
    if (!store.indexNames.contains("userConversation")) {
      store.createIndex("userConversation", ["userId", "conversationId"], { unique: false });
    }
  };
  request.onsuccess = () => resolve(request.result);
  request.onerror = () => reject(request.error);
});

const run = (db, mode, operation) => new Promise((resolve, reject) => {
  const transaction = db.transaction(STORE_NAME, mode);
  const request = operation(transaction.objectStore(STORE_NAME));
  let result;
  if (request) {
    request.onsuccess = () => {
      result = request.result;
    };
    request.onerror = () => reject(request.error);
  }
  transaction.oncomplete = () => {
    db.close();
    resolve(result);
  };
  transaction.onerror = () => {
    db.close();
    reject(transaction.error);
  };
});

const toRecord = (userId, conversationId, message) => ({
  cacheId: userId + ":" + conversationId + ":" + (message.clientMsgId || message.messageId || message.id),
  userId,
  conversationId,
  messageId: message.messageId || (message.clientMsgId ? "" : message.id) || "",
  senderId: message.senderId || "",
  seq: Number(message.seq) || 0,
  convType: Number(message.convType) || 1,
  cType: Number(message.cType) || 1,
  content: message.content || "",
  sendTime: Number(message.sendTime) || Date.now(),
  clientMsgId: message.clientMsgId || "",
  status: message.status || "sent",
  fileName: message.fileName || "",
  fileSize: message.fileSize || 0,
  mediaUrl: message.mediaUrl || "",
  thumbUrl: message.thumbUrl || "",
  fileId: message.fileId || "",
  width: Number(message.width) || 0,
  height: Number(message.height) || 0,
  durationMs: Number(message.durationMs) || 0,
});

export const readMessages = async (userId, conversationId) => {
  if (!userId || !conversationId) return [];
  const db = await openDb();
  const records = await run(db, "readonly", (store) =>
    store.index("userConversation").getAll([userId, conversationId]));
  return (records || [])
    .sort((a, b) => (a.seq || a.sendTime) - (b.seq || b.sendTime))
    .map((record) => ({ ...record, id: record.messageId || record.clientMsgId }));
};

export const writeMessages = async (userId, conversationId, messages) => {
  if (!userId || !conversationId || !messages?.length) return;
  const db = await openDb();
  await run(db, "readwrite", (store) => {
    messages.forEach((message) => store.put(toRecord(userId, conversationId, message)));
  });
};

export const deleteMessages = async (userId, conversationId) => {
  if (!userId || !conversationId) return;
  const db = await openDb();
  const records = await run(db, "readonly", (store) =>
    store.index("userConversation").getAll([userId, conversationId]));
  const writeDb = await openDb();
  await run(writeDb, "readwrite", (store) => {
    records.forEach((record) => store.delete(record.cacheId));
  });
};
