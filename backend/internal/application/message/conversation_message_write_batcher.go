package message

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	outboxport "IM_backend/internal/application/ports/outbox"
	messagecache "IM_backend/internal/application/ports/persistence/cache/message"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	conversationvo "IM_backend/internal/domain/conversation/value_object"
	fileentity "IM_backend/internal/domain/file/entity"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	"IM_backend/internal/shared/diagnostics"
	"IM_backend/internal/shared/protocol"
)

var (
	ErrConversationMessageBatchFull     = errors.New("会话消息批处理队列已满")
	ErrConversationMessageBatcherClosed = errors.New("会话消息批处理器已关闭")
)

// ConversationMessageWriteBatchOptions 配置按会话聚合的短窗口消息写入。
type ConversationMessageWriteBatchOptions struct {
	Linger      time.Duration
	MaxMessages int
	ShardCount  int
	MaxPending  int
}

type conversationMessageWriteRequest struct {
	ctx        context.Context
	dto        SendMessageDTO
	result     chan messageWriteResult
	enqueuedAt time.Time
}

type conversationMessageWriteBatch struct {
	conversationID string
	requests       []*conversationMessageWriteRequest
	flushedAt      time.Time
	readyAt        time.Time
}

type messageWriteResult struct {
	ack *MessageAckDTO
	err error
}

type preparedMessageWrite struct {
	ctx            context.Context
	dto            SendMessageDTO
	conversationID string
	requestHash    string
	message        *messageentity.Message
	userConv       *conversationentity.UserConversation
	isDanmaku      bool
	senderUsername string
	event          protocol.MessageEvent
	mediaWriter    func(context.Context, any) error
	request        *conversationMessageWriteRequest
}

type conversationMessageWriteBatcher struct {
	app     *MessageApplication
	shards  []*conversationMessageWriteShard
	done    chan struct{}
	once    sync.Once
	stateMu sync.RWMutex
	closed  bool
}

type conversationMessageWriteShard struct {
	mu             sync.Mutex
	pending        map[string][]*conversationMessageWriteRequest
	flushRequested map[string]struct{}
	pendingSize    int
	maxPending     int
	maxMessages    int
	flushSignal    chan struct{}
	ticker         *time.Ticker
	readyQueue     chan *conversationMessageWriteBatch
	stopped        chan struct{}
	parent         *conversationMessageWriteBatcher
}

func newConversationMessageWriteBatcher(
	app *MessageApplication,
	options ConversationMessageWriteBatchOptions,
) *conversationMessageWriteBatcher {
	if options.Linger <= 0 {
		options.Linger = 5 * time.Millisecond
	}
	if options.MaxMessages <= 0 {
		options.MaxMessages = 50
	}
	if options.ShardCount <= 0 {
		options.ShardCount = 16
	}
	if options.MaxPending <= 0 {
		options.MaxPending = 100000
	}

	batcher := &conversationMessageWriteBatcher{
		app:    app,
		shards: make([]*conversationMessageWriteShard, options.ShardCount),
		done:   make(chan struct{}),
	}
	perShardPending := options.MaxPending / options.ShardCount
	if perShardPending <= 0 {
		perShardPending = 1
	}
	readyQueueSize := perShardPending / options.MaxMessages
	if readyQueueSize <= 0 {
		readyQueueSize = 1
	}
	for i := range batcher.shards {
		shard := &conversationMessageWriteShard{
			pending:        make(map[string][]*conversationMessageWriteRequest),
			flushRequested: make(map[string]struct{}),
			maxPending:     perShardPending,
			maxMessages:    options.MaxMessages,
			flushSignal:    make(chan struct{}, 1),
			ticker:         time.NewTicker(options.Linger),
			readyQueue:     make(chan *conversationMessageWriteBatch, readyQueueSize),
			stopped:        make(chan struct{}),
			parent:         batcher,
		}
		batcher.shards[i] = shard
		go shard.run()
		go shard.writeLoop()
	}
	return batcher
}

func (batcher *conversationMessageWriteBatcher) submit(ctx context.Context, dto SendMessageDTO) (*MessageAckDTO, error) {
	if batcher == nil || batcher.app == nil {
		return nil, errors.New("会话消息批处理器未配置")
	}
	conversationID := conversationentity.GetConversationID(dto.SenderID, dto.ReceiverID, dto.ConversationType)
	request := &conversationMessageWriteRequest{
		ctx:        ctx,
		dto:        dto,
		result:     make(chan messageWriteResult, 1),
		enqueuedAt: time.Now(),
	}
	batcher.stateMu.RLock()
	if batcher.closed {
		batcher.stateMu.RUnlock()
		return &MessageAckDTO{ClientMessageID: dto.ClientMessageID, Status: string(protocol.AckStatusFailed)}, ErrConversationMessageBatcherClosed
	}
	err := batcher.shardFor(conversationID).put(conversationID, request)
	batcher.stateMu.RUnlock()
	if err != nil {
		return &MessageAckDTO{ClientMessageID: dto.ClientMessageID, Status: string(protocol.AckStatusFailed)}, err
	}

	select {
	case result := <-request.result:
		return result.ack, result.err
	case <-ctx.Done():
		return &MessageAckDTO{ClientMessageID: dto.ClientMessageID, Status: string(protocol.AckStatusFailed)}, ctx.Err()
	}
}

func (batcher *conversationMessageWriteBatcher) shardFor(conversationID string) *conversationMessageWriteShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(conversationID))
	return batcher.shards[int(h.Sum32())%len(batcher.shards)]
}

func (batcher *conversationMessageWriteBatcher) close(ctx context.Context) {
	if batcher == nil {
		return
	}
	batcher.once.Do(func() {
		batcher.stateMu.Lock()
		batcher.closed = true
		close(batcher.done)
		batcher.stateMu.Unlock()
	})
	if ctx == nil {
		ctx = context.Background()
	}
	for _, shard := range batcher.shards {
		select {
		case <-shard.stopped:
		case <-ctx.Done():
			return
		}
	}
}

func (shard *conversationMessageWriteShard) put(conversationID string, request *conversationMessageWriteRequest) error {
	shard.mu.Lock()
	if shard.pendingSize >= shard.maxPending {
		shard.mu.Unlock()
		return ErrConversationMessageBatchFull
	}
	shard.pending[conversationID] = append(shard.pending[conversationID], request)
	shard.pendingSize++
	shouldFlush := len(shard.pending[conversationID]) >= shard.maxMessages
	if shouldFlush {
		shard.flushRequested[conversationID] = struct{}{}
	}
	shard.mu.Unlock()

	if shouldFlush {
		select {
		case shard.flushSignal <- struct{}{}:
		default:
		}
	}
	return nil
}

func (shard *conversationMessageWriteShard) run() {
	defer func() {
		shard.ticker.Stop()
		shard.flush()
		close(shard.readyQueue)
	}()
	for {
		select {
		case <-shard.parent.done:
			return
		case <-shard.flushSignal:
			shard.flushRequestedConversations()
		case <-shard.ticker.C:
			shard.flush()
		}
	}
}

func (shard *conversationMessageWriteShard) flush() {
	shard.mu.Lock()
	if len(shard.pending) == 0 {
		shard.flushRequested = make(map[string]struct{})
		shard.mu.Unlock()
		return
	}
	pending := shard.pending
	shard.pending = make(map[string][]*conversationMessageWriteRequest)
	shard.flushRequested = make(map[string]struct{})
	shard.mu.Unlock()

	for conversationID, requests := range pending {
		shard.enqueueReadyBatches(conversationID, requests)
	}
}

func (shard *conversationMessageWriteShard) flushRequestedConversations() {
	shard.mu.Lock()
	requested := shard.flushRequested
	shard.flushRequested = make(map[string]struct{})
	pending := make(map[string][]*conversationMessageWriteRequest, len(requested))
	for conversationID := range requested {
		if requests := shard.pending[conversationID]; len(requests) > 0 {
			pending[conversationID] = requests
			delete(shard.pending, conversationID)
		}
	}
	shard.mu.Unlock()

	for conversationID, requests := range pending {
		shard.enqueueReadyBatches(conversationID, requests)
	}
}

func (shard *conversationMessageWriteShard) enqueueReadyBatches(
	conversationID string,
	requests []*conversationMessageWriteRequest,
) {
	flushedAt := time.Now()
	processConversationWriteRequestsInChunks(requests, shard.maxMessages, func(batch []*conversationMessageWriteRequest) {
		shard.readyQueue <- &conversationMessageWriteBatch{
			conversationID: conversationID,
			requests:       batch,
			flushedAt:      flushedAt,
			readyAt:        time.Now(),
		}
	})
}

func (shard *conversationMessageWriteShard) writeLoop() {
	defer close(shard.stopped)
	for batch := range shard.readyQueue {
		if batch == nil || len(batch.requests) == 0 {
			continue
		}
		shard.parent.app.processConversationWriteBatch(batch.requests[0].ctx, batch)
		shard.mu.Lock()
		shard.pendingSize -= len(batch.requests)
		shard.mu.Unlock()
	}
}

func processConversationWriteRequestsInChunks(
	requests []*conversationMessageWriteRequest,
	maxMessages int,
	process func([]*conversationMessageWriteRequest),
) {
	if maxMessages <= 0 {
		maxMessages = len(requests)
	}
	for len(requests) > 0 {
		size := min(maxMessages, len(requests))
		batch := requests[:size]
		process(batch)
		requests = requests[size:]
	}
}

func (ma *MessageApplication) processConversationWriteBatch(
	batchCtx context.Context,
	batch *conversationMessageWriteBatch,
) {
	if batch == nil {
		return
	}
	requests := batch.requests
	startedAt := time.Now()
	queueWait := time.Duration(0)
	flushWait := time.Duration(0)
	readyQueueWait := time.Duration(0)
	if len(requests) > 0 {
		queueWait = startedAt.Sub(requests[0].enqueuedAt)
		if !batch.flushedAt.IsZero() {
			flushWait = batch.flushedAt.Sub(requests[0].enqueuedAt)
		}
		if !batch.readyAt.IsZero() {
			readyQueueWait = startedAt.Sub(batch.readyAt)
		}
	}
	prepareStartedAt := time.Now()
	prepared := make([]*preparedMessageWrite, 0, len(requests))
	for _, request := range requests {
		item, immediateAck, err := ma.prepareMessageWrite(request.ctx, request.dto)
		if err != nil || immediateAck != nil {
			respondMessageWrite(request, immediateAck, err)
			continue
		}
		item.request = request
		prepared = append(prepared, item)
	}
	prepareDuration := time.Since(prepareStartedAt)
	if len(prepared) == 0 {
		diagnostics.Logf("stage=message_write_batch conversation_id=%s requested=%d prepared=0 queue_wait_us=%d prepare_us=%d total_us=%d outcome=skipped",
			batch.conversationID, len(requests), queueWait.Microseconds(), prepareDuration.Microseconds(), time.Since(startedAt).Microseconds())
		return
	}

	// 保留请求上下文的值，但客户端断开不应中断已受理的批量持久化。
	ctx := context.WithoutCancel(prepared[0].ctx)
	if batchCtx != nil {
		ctx = context.WithoutCancel(batchCtx)
	}
	persistStartedAt := time.Now()
	if err := ma.persistPreparedBatch(ctx, prepared); err != nil {
		persistDuration := time.Since(persistStartedAt)
		fallbackStartedAt := time.Now()
		// 批事务整体回滚后逐条回退，避免幂等冲突或文件删除影响同批其他消息。
		for _, item := range prepared {
			ack, singleErr := ma.handleSendMessageSingle(item.request.ctx, item.request.dto)
			respondMessageWrite(item.request, ack, singleErr)
		}
		diagnostics.Logf("stage=message_write_batch conversation_id=%s requested=%d prepared=%d queue_wait_us=%d flush_wait_us=%d ready_queue_wait_us=%d prepare_us=%d persist_us=%d fallback_us=%d total_us=%d outcome=fallback error=%q",
			batch.conversationID, len(requests), len(prepared), queueWait.Microseconds(), flushWait.Microseconds(), readyQueueWait.Microseconds(),
			prepareDuration.Microseconds(), persistDuration.Microseconds(), time.Since(fallbackStartedAt).Microseconds(), time.Since(startedAt).Microseconds(), err.Error())
		return
	}
	persistDuration := time.Since(persistStartedAt)

	finalizeStartedAt := time.Now()
	for _, item := range prepared {
		ack, err := ma.finalizePreparedMessage(ctx, item, nil)
		respondMessageWrite(item.request, ack, err)
	}
	diagnostics.Logf("stage=message_write_batch conversation_id=%s requested=%d prepared=%d queue_wait_us=%d flush_wait_us=%d ready_queue_wait_us=%d prepare_us=%d persist_us=%d finalize_us=%d total_us=%d outcome=committed",
		batch.conversationID, len(requests), len(prepared), queueWait.Microseconds(), flushWait.Microseconds(), readyQueueWait.Microseconds(),
		prepareDuration.Microseconds(), persistDuration.Microseconds(), time.Since(finalizeStartedAt).Microseconds(), time.Since(startedAt).Microseconds())
}

func respondMessageWrite(request *conversationMessageWriteRequest, ack *MessageAckDTO, err error) {
	if request == nil {
		return
	}
	request.result <- messageWriteResult{ack: ack, err: err}
}

func (ma *MessageApplication) prepareMessageWrite(ctx context.Context, dto SendMessageDTO) (*preparedMessageWrite, *MessageAckDTO, error) {
	conversationID := conversationentity.GetConversationID(dto.SenderID, dto.ReceiverID, dto.ConversationType)
	dto.ConversationID = conversationID
	fail := func(err error) (*preparedMessageWrite, *MessageAckDTO, error) {
		return nil, &MessageAckDTO{ClientMessageID: dto.ClientMessageID, Status: string(protocol.AckStatusFailed)}, err
	}
	if err := ma.validateMessageCType(dto); err != nil {
		return fail(err)
	}
	requestHash, err := buildMessageRequestHash(dto)
	if err != nil {
		return fail(err)
	}
	if dto.ClientMessageID != "" && ma.messageCache != nil {
		if entry, err := ma.messageCache.GetDedupEntry(ctx, dto.SenderID, dto.ClientMessageID); err == nil && entry.MessageID != "" {
			if entry.RequestHash != requestHash {
				return fail(messageentity.ErrClientMessageConflict)
			}
			if entry.ConversationID != "" && entry.Seq > 0 {
				return nil, &MessageAckDTO{ClientMessageID: dto.ClientMessageID, ConversationID: entry.ConversationID, MessageID: entry.MessageID, Seq: entry.Seq, SendTime: entry.SendTime, AttachmentID: entry.AttachmentID, Status: string(protocol.AckStatusSent)}, nil
			}
		}
	}
	if err := ma.checkConvMember(ctx, dto); err != nil {
		return fail(err)
	}
	if err := ma.normalizeMediaDTO(ctx, &dto); err != nil {
		return fail(err)
	}
	messageID, err := ma.idGenerator.Generate()
	if err != nil {
		return fail(err)
	}
	message := messageentity.NewMessage(messageID, conversationID, dto.SenderID, 0, messagevo.CType(dto.Type), dto.Content, dto.VideoID, dto.VideoTime)
	if dto.ClientMessageID != "" {
		clientMessageID := dto.ClientMessageID
		message.ClientMsgId = &clientMessageID
	}
	message.RequestHash = requestHash
	mediaWriter, err := ma.buildMediaWriter(&dto, messageID)
	if err != nil {
		return fail(err)
	}
	if ma.txManager == nil || ma.messageOutboxRepository == nil {
		return fail(errors.New("消息出箱组件未配置"))
	}
	if op, ok := ctx.Value("op").(string); !ok || op == "" {
		return fail(ErrUnknown)
	}
	usernameLookupStartedAt := time.Now()
	senderUsername := ma.getUsername(dto.SenderID)
	diagnostics.Logf("stage=message_sender_username_lookup duration_us=%d outcome=completed", time.Since(usernameLookupStartedAt).Microseconds())
	return &preparedMessageWrite{
		ctx:            ctx,
		dto:            dto,
		conversationID: conversationID,
		requestHash:    requestHash,
		message:        message,
		userConv:       conversationentity.BuildUserConversation(dto.SenderID, conversationID, 0),
		isDanmaku:      dto.ConversationType == int(conversationvo.RoomChat) && dto.VideoTime != nil,
		senderUsername: senderUsername,
		event:          protocol.MessageEvent{MessageId: messageID, ConversationId: conversationID, SenderId: dto.SenderID, SenderUsername: senderUsername, RecvId: dto.ReceiverID, ConvType: protocol.ConvType(dto.ConversationType), CType: dto.Type, Content: dto.Content, VideoId: dto.VideoID, VideoTime: dto.VideoTime, SendTime: message.SendTime, ClientMsgId: dto.ClientMessageID, Status: int8(message.Status), AttachmentId: dto.AttachmentID, Width: dto.Width, Height: dto.Height, DurationMs: dto.DurationMs, StickerId: dto.StickerID, PackId: dto.PackID, HasVideoTime: dto.VideoTime != nil},
		mediaWriter:    mediaWriter,
	}, nil, nil
}

func (ma *MessageApplication) persistPreparedBatch(ctx context.Context, entries []*preparedMessageWrite) error {
	if len(entries) == 0 {
		return nil
	}
	conversationID := entries[0].conversationID
	lastMessageID := entries[len(entries)-1].message.MessageId
	var reserveSequenceDuration time.Duration
	var createMessagesDuration time.Duration
	var updateConversationsDuration time.Duration
	var createOutboxDuration time.Duration
	transactionStartedAt := time.Now()
	err := ma.txManager.WithinTransaction(ctx, func(tx any) error {
		convRepo := ma.conversationRepository.WithTx(tx)
		stepStartedAt := time.Now()
		baseSeq, err := convRepo.ReserveSequenceRange(ctx, conversationID, lastMessageID, len(entries))
		reserveSequenceDuration = time.Since(stepStartedAt)
		if err != nil {
			if errors.Is(err, conversationentity.ErrConversationNotCreated) {
				return ErrConversationNotFound
			}
			return ErrConversationSequenceUpdate
		}
		mediaFiles := make(map[*preparedMessageWrite]*fileentity.File, len(entries))
		messages := make([]*messageentity.Message, 0, len(entries))
		userConversations := make([]*conversationentity.UserConversation, 0, len(entries))
		for index, entry := range entries {
			entry.message.Seq = baseSeq + int64(index) + 1
			entry.event.Seq = entry.message.Seq
			entry.userConv.UpdateReadSeq(entry.message.Seq)
			mediaFile, err := ma.preparePreparedMessageInTx(ctx, tx, entry)
			if err != nil {
				return err
			}
			mediaFiles[entry] = mediaFile
			messages = append(messages, entry.message)
			if !entry.isDanmaku {
				userConversations = append(userConversations, entry.userConv)
			}
		}
		stepStartedAt = time.Now()
		if err := ma.messageRepository.WithTx(tx).CreateNewMessages(ctx, messages); err != nil {
			createMessagesDuration = time.Since(stepStartedAt)
			return err
		}
		createMessagesDuration = time.Since(stepStartedAt)
		stepStartedAt = time.Now()
		if err := ma.userConversationRepository.WithTx(tx).BatchUpdateReadSeq(ctx, userConversations); err != nil {
			updateConversationsDuration = time.Since(stepStartedAt)
			return err
		}
		updateConversationsDuration = time.Since(stepStartedAt)

		outboxes := make([]*outboxport.Entry, 0, len(entries)*2)
		for _, entry := range entries {
			if entry.mediaWriter != nil {
				if err := entry.mediaWriter(ctx, tx); err != nil {
					return err
				}
			}
			entryOutboxes, err := ma.buildPreparedMessageOutboxes(entry, mediaFiles[entry])
			if err != nil {
				return err
			}
			outboxes = append(outboxes, entryOutboxes...)
		}
		stepStartedAt = time.Now()
		if err := ma.messageOutboxRepository.WithTx(tx).CreateBatch(ctx, outboxes); err != nil {
			createOutboxDuration = time.Since(stepStartedAt)
			return err
		}
		createOutboxDuration = time.Since(stepStartedAt)
		return nil
	})
	outcome := "committed"
	if err != nil {
		outcome = "error"
	}
	diagnostics.Logf("stage=mysql_message_transaction conversation_id=%s messages=%d reserve_seq_us=%d create_messages_us=%d update_conversations_us=%d create_outbox_us=%d transaction_us=%d outcome=%s",
		conversationID, len(entries), reserveSequenceDuration.Microseconds(), createMessagesDuration.Microseconds(),
		updateConversationsDuration.Microseconds(), createOutboxDuration.Microseconds(), time.Since(transactionStartedAt).Microseconds(), outcome)
	if err == nil {
		ma.notifyProducer()
	}
	return err
}

func (ma *MessageApplication) preparePreparedMessageInTx(ctx context.Context, tx any, entry *preparedMessageWrite) (*fileentity.File, error) {
	if kind := messagevo.CType(entry.dto.Type); kind == messagevo.Image || kind == messagevo.Video || kind == messagevo.File {
		lockedFile, err := ma.fileRepository.WithTx(tx).FindUploadedFileByIDAndUploaderForUpdate(ctx, entry.dto.FileID, entry.dto.SenderID)
		if err != nil {
			return nil, err
		}
		if lockedFile == nil {
			return nil, fmt.Errorf("file is missing or being deleted: %s", entry.dto.FileID)
		}
		entry.dto.FileName, entry.dto.FileSize, entry.dto.MimeType = lockedFile.FileName, lockedFile.Size, lockedFile.ContentType
		entry.dto.Content, entry.message.Content, entry.event.Content = lockedFile.FileName, lockedFile.FileName, lockedFile.FileName
		return lockedFile, nil
	}
	return nil, nil
}

func (ma *MessageApplication) buildPreparedMessageOutboxes(entry *preparedMessageWrite, mediaFile *fileentity.File) ([]*outboxport.Entry, error) {
	payload, err := json.Marshal(entry.event)
	if err != nil {
		return nil, err
	}
	envelope, err := json.Marshal(protocol.Envelope{From: entry.event.SenderId, To: entry.event.RecvId, Payload: payload})
	if err != nil {
		return nil, err
	}
	result := []*outboxport.Entry{{EventType: string(protocol.EventTypeSendMessage), MessageKey: entry.conversationID, Payload: envelope}}
	if mediaFile == nil || entry.dto.AttachmentID == "" {
		return result, nil
	}
	warmupPayload, err := json.Marshal(protocol.FileCardWarmupEvent{AttachmentID: entry.dto.AttachmentID, MessageID: entry.message.MessageId, ConversationID: entry.conversationID, FileID: mediaFile.FileId, ObjectKey: mediaFile.ObjectKey, FileName: mediaFile.FileName, ContentType: mediaFile.ContentType, Size: mediaFile.Size, Status: mediaFile.Status, CType: entry.dto.Type, AttachmentExpireAt: time.Now().Add(time.Duration(ma.config.Message.AttachmentTTLSeconds) * time.Second).UnixMilli()})
	if err != nil {
		return nil, err
	}
	warmupEnvelope, err := json.Marshal(protocol.Envelope{From: entry.event.SenderId, To: mediaFile.FileId, Payload: warmupPayload})
	if err != nil {
		return nil, err
	}
	return append(result, &outboxport.Entry{EventType: string(protocol.EventFileCardWarmup), MessageKey: mediaFile.FileId, Payload: warmupEnvelope}), nil
}

func (ma *MessageApplication) finalizePreparedMessage(ctx context.Context, entry *preparedMessageWrite, persistErr error) (*MessageAckDTO, error) {
	if persistErr == nil && entry.dto.ClientMessageID != "" && ma.messageCache != nil {
		_, _ = ma.messageCache.SetDedupEntry(ctx, entry.dto.SenderID, entry.dto.ClientMessageID, messagecache.DedupEntry{MessageID: entry.message.MessageId, RequestHash: entry.requestHash, ConversationID: entry.conversationID, Seq: entry.message.Seq, AttachmentID: entry.dto.AttachmentID, SendTime: entry.message.SendTime, SenderUsername: entry.senderUsername}, 5*time.Minute)
	}
	if persistErr != nil {
		return &MessageAckDTO{ClientMessageID: entry.dto.ClientMessageID, MessageID: entry.message.MessageId, Status: string(protocol.AckStatusFailed)}, persistErr
	}
	return &MessageAckDTO{ClientMessageID: entry.dto.ClientMessageID, ConversationID: entry.conversationID, MessageID: entry.message.MessageId, AttachmentID: entry.dto.AttachmentID, Seq: entry.message.Seq, SendTime: entry.message.SendTime, Status: string(protocol.AckStatusSent)}, nil
}
