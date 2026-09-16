/**
 * 只负责按本地连续边界补偿实时消息缺口。
 * 历史消息展示完全由 IndexedDB 游标查询负责，不在这里回源。
 */
export const createMessageSyncService = ({ syncMessages }) => {
  const syncInFlight = new Map();

  const syncAfter = (conversationId, afterSeq, scope = "") => {
    const key = `${scope}:${conversationId}`;
    const existing = syncInFlight.get(key);
    if (existing) return existing;

    const request = syncMessages(
      conversationId,
      Math.max(Number(afterSeq) || 0, 0),
    ).finally(() => syncInFlight.delete(key));
    syncInFlight.set(key, request);
    return request;
  };

  return { syncAfter };
};
