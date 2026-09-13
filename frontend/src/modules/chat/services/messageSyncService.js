const DEFAULT_HISTORY_PAGE_SIZE = 30;

/** 返回没有消息的统一历史分页结果。 */
const emptyHistoryPage = () => ({
  messages: [],
  hasMore: false,
  nextCursor: -1,
});

/** 创建本地优先的消息同步服务，负责历史分页、缺口修复和增量同步去重。 */
export const createMessageSyncService = ({
  getHistory,
  getMessagesBySeqs,
  syncMessages,
  queryMessagesByCursor,
  insertMessages,
  canContinue = () => true,
}) => {
  const syncInFlight = new Map();
  const historyInFlight = new Map();
  const historyMetadata = new Map();

  /** 找出本地当前历史页中缺失的连续 seq，供服务端批量补齐。 */
  const findMissingSeqs = (messages, cursor, latestSeq, limit) => {
    const endSeq = Number(cursor) > 0
      ? Number(cursor) - 1
      : Number(latestSeq) || 0;
    if (endSeq <= 0) return [];

    const startSeq = Math.max(1, endSeq - limit + 1);
    const existingSeqs = new Set(
      (messages || [])
        .map((message) => Number(message.seq))
        .filter((seq) => Number.isSafeInteger(seq) && seq > 0),
    );
    const missing = [];
    for (let seq = startSeq; seq <= endSeq; seq += 1) {
      if (!existingSeqs.has(seq)) missing.push(seq);
    }
    return missing;
  };

  /** 查询本地历史页；发现 seq 断点时批量回源并回填后重新查询。 */
  const queryLocalPage = async (
    conversationId,
    cursor,
    limit,
    latestSeq,
    requestCanContinue,
    insertContext,
  ) => {
    let page = await queryMessagesByCursor({ conversationId, cursor, limit });
    const missingSeqs = page.messages.length
      ? findMissingSeqs(page.messages, cursor, latestSeq, limit)
      : [];

    if (missingSeqs.length) {
      const data = await getMessagesBySeqs(conversationId, missingSeqs);
      if (!requestCanContinue()) return page;
      const missingMessages = data?.messages || [];
      if (missingMessages.length) {
        await insertMessages(missingMessages, insertContext);
        page = await queryMessagesByCursor({ conversationId, cursor, limit });
      }
    }

    return page;
  };

  /** 请求远端历史页，并按请求参数复用进行中的请求，避免重复查询。 */
  const remoteHistoryPage = (conversationId, cursor, limit, scope) => {
    const key = `${scope}:${conversationId}:${Number(cursor) || 0}:${limit}`;
    const existing = historyInFlight.get(key);
    if (existing) return existing;

    const request = getHistory(conversationId, cursor, limit)
      .finally(() => historyInFlight.delete(key));
    historyInFlight.set(key, request);
    return request;
  };

  /** 加载历史消息，优先使用本地缓存，无法确认完整性时再请求服务端。 */
  const loadHistoryPage = async ({
    conversationId,
    cursor = 0,
    limit = DEFAULT_HISTORY_PAGE_SIZE,
    latestSeq = 0,
    scope = "",
    insertContext,
    requestCanContinue = canContinue,
  }) => {
    const page = await queryLocalPage(
      conversationId,
      cursor,
      limit,
      latestSeq,
      requestCanContinue,
      insertContext,
    );
    const metadataKey = `${scope}:${conversationId}:${Number(cursor) || 0}:${limit}`;
    const knownMetadata = historyMetadata.get(metadataKey);

    if (knownMetadata) {
      return { ...page, ...knownMetadata, fromCache: true };
    }

    // 本地没有额外记录时，必须向服务端确认是否还有更早消息。
    // 否则仅凭本地数量无法区分“最后一页”和“缓存尚未覆盖”。
    if (page.hasMore) return { ...page, fromCache: true };

    const remote = await remoteHistoryPage(conversationId, cursor, limit, scope);
    if (!requestCanContinue()) return emptyHistoryPage();
    const remoteMessages = remote?.messages || [];
    if (remoteMessages.length) {
      await insertMessages(remoteMessages, insertContext);
    }

    historyMetadata.set(metadataKey, {
      hasMore: Boolean(remote?.hasMore),
      nextCursor: Number.isFinite(Number(remote?.nextCursor))
        ? Number(remote.nextCursor)
        : -1,
    });

    const refreshed = remoteMessages.length
      ? await queryLocalPage(
        conversationId,
        cursor,
        limit,
        latestSeq,
        requestCanContinue,
        insertContext,
      )
      : emptyHistoryPage();

    return {
      messages: refreshed.messages.length ? refreshed.messages : remoteMessages,
      hasMore: Boolean(remote?.hasMore),
      nextCursor: Number.isFinite(Number(remote?.nextCursor))
        ? Number(remote.nextCursor)
        : refreshed.nextCursor,
      fromCache: false,
    };
  };

  /** 从指定 seq 之后同步增量消息，并合并同一会话的并发同步请求。 */
  const syncAfter = (conversationId, afterSeq, scope = "") => {
    const key = `${scope}:${conversationId}`;
    const existing = syncInFlight.get(key);
    if (existing) return existing;

    const request = syncMessages(conversationId, Math.max(Number(afterSeq) || 0, 0))
      .finally(() => syncInFlight.delete(key));
    syncInFlight.set(key, request);
    return request;
  };

  return {
    loadHistoryPage,
    syncAfter,
    invalidateHistory: (conversationId, scope = "") => {
      const prefix = `${scope}:${conversationId}:`;
      for (const key of historyMetadata.keys()) {
        if (key.startsWith(prefix)) historyMetadata.delete(key);
      }
    },
  };
};
