package websocket

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/shared/diagnostics"
	"IM_backend/internal/shared/protocol"
	wspb "IM_backend/internal/transport/ws/pb"

	gorilla "github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var (
	ErrSessionClosed     = errors.New("实时通道会话已关闭")
	ErrOutboundQueueFull = errors.New("实时通道发送队列已满")
)

type Message struct {
	Op                string
	Data              []byte
	inboundEnqueuedAt time.Time
}

type OutboundItem struct {
	Message Message
	Policy  AppendPolicy
}

type MessageHandler func(context.Context, *Session, string, []byte)

type Session struct {
	conn *gorilla.Conn
	ctx  context.Context

	identity SessionIdentity

	idGenerator *snow.Generator
	batchConfig MessageBatchConfig
	maxReadSize int64
	outbound    chan OutboundItem
	inbound     chan Message

	cancel context.CancelFunc

	closeOnce      sync.Once
	shutdownOnce   sync.Once
	writeDone      chan struct{}
	stateMu        sync.RWMutex
	accepting      bool
	outboundClosed bool
	activityMu     sync.RWMutex
	idle           int64
}

func NewSession(
	ctx context.Context,
	cancel context.CancelFunc,
	conn *gorilla.Conn,
	identity SessionIdentity,
	maxBufferSize int,
	idGenerator *snow.Generator,
	batchConfig MessageBatchConfig,
) *Session {
	if maxBufferSize <= 0 {
		maxBufferSize = DefaultBatchConfig().ReadyQueueSize
	}

	return &Session{
		conn:        conn,
		ctx:         ctx,
		cancel:      cancel,
		identity:    identity,
		idle:        time.Now().UnixMilli(),
		idGenerator: idGenerator,
		batchConfig: batchConfig.withDefaults(),
		outbound:    make(chan OutboundItem, maxBufferSize),
		inbound:     make(chan Message, maxBufferSize),
		writeDone:   make(chan struct{}),
		accepting:   true,
	}
}

func (s *Session) marshalBatch(batch *MessageBatch) ([]byte, error) {
	frames := make([]*wspb.WsFrame, 0, len(batch.Messages))

	for _, message := range batch.Messages {
		frames = append(frames, &wspb.WsFrame{
			Op:   message.Op,
			Data: message.Data,
		})
	}

	batchData, err := proto.Marshal(&wspb.WsBatch{
		Frames: frames,
	})

	if err != nil {
		return nil, fmt.Errorf("序列化消息批次失败: %w", err)
	}

	payload, err := proto.Marshal(&wspb.WsFrame{
		Op:   protocol.EventMessageBatch,
		Data: batchData,
	})

	if err != nil {
		return nil, fmt.Errorf("序列化批量消息帧失败: %w", err)
	}

	return payload, nil
}

func (s *Session) sendBatch(batch *MessageBatch, writeWait int) error {
	if err := batch.StartSending(); err != nil {
		return err
	}
	firstSeq, lastSeq, messageFrames := outboundMessageSeqRange(batch)

	marshalStartedAt := time.Now()
	payload, err := s.marshalBatch(batch)
	marshalDuration := time.Since(marshalStartedAt)
	if err != nil {
		_ = batch.MarkFailed(err)
		diagnostics.Logf("stage=ws_outbound_batch session_id=%s batch_id=%s messages=%d message_frames=%d first_seq=%d last_seq=%d bytes=%d marshal_us=%d outcome=marshal_error error=%q",
			s.SessionID(), batch.Id, batch.MessageCount(), messageFrames, firstSeq, lastSeq, batch.Bytes, marshalDuration.Microseconds(), err.Error())
		return err
	}

	writeStartedAt := time.Now()
	if err := s.write(gorilla.BinaryMessage, payload, writeWait); err != nil {
		_ = batch.MarkFailed(err)
		diagnostics.Logf("stage=ws_outbound_batch session_id=%s batch_id=%s messages=%d message_frames=%d first_seq=%d last_seq=%d bytes=%d linger_us=%d queue_wait_us=%d marshal_us=%d socket_write_us=%d outcome=write_error error=%q",
			s.SessionID(), batch.Id, batch.MessageCount(), messageFrames, firstSeq, lastSeq, batch.Bytes, batch.SealedAt.Sub(batch.CreateAt).Microseconds(),
			batch.SendingAt.Sub(batch.SealedAt).Microseconds(), marshalDuration.Microseconds(), time.Since(writeStartedAt).Microseconds(), err.Error())
		return err
	}

	markErr := batch.MarkSent()
	diagnostics.Logf("stage=ws_outbound_batch session_id=%s batch_id=%s messages=%d message_frames=%d first_seq=%d last_seq=%d bytes=%d linger_us=%d queue_wait_us=%d marshal_us=%d socket_write_us=%d total_us=%d outcome=%s",
		s.SessionID(), batch.Id, batch.MessageCount(), messageFrames, firstSeq, lastSeq, batch.Bytes, batch.SealedAt.Sub(batch.CreateAt).Microseconds(),
		batch.SendingAt.Sub(batch.SealedAt).Microseconds(), marshalDuration.Microseconds(), time.Since(writeStartedAt).Microseconds(),
		time.Since(batch.SendingAt).Microseconds(), sessionBatchOutcome(markErr))
	return markErr
}

func outboundMessageSeqRange(batch *MessageBatch) (int64, int64, int) {
	if !diagnostics.Enabled() || batch == nil {
		return 0, 0, 0
	}
	var firstSeq, lastSeq int64
	count := 0
	for _, item := range batch.Messages {
		if item.Op != string(protocol.EventTypeSendMessage) {
			continue
		}
		var event wspb.MessageEvent
		if err := proto.Unmarshal(item.Data, &event); err != nil || event.GetSeq() <= 0 {
			continue
		}
		seq := event.GetSeq()
		if count == 0 || seq < firstSeq {
			firstSeq = seq
		}
		if count == 0 || seq > lastSeq {
			lastSeq = seq
		}
		count++
	}
	return firstSeq, lastSeq, count
}

func sessionBatchOutcome(err error) string {
	if err != nil {
		return "state_error"
	}
	return "sent"
}

func (s *Session) Start(pongWait, pingPeriod, writeWait int, handler MessageHandler, onClose func(*Session)) {
	go s.readLoop(pongWait, handler, onClose)
	go s.writeLoop(pingPeriod, writeWait)
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.stopAccepting()
		s.cancel()
		if s.conn != nil {
			_ = s.conn.WriteControl(
				gorilla.CloseMessage,
				gorilla.FormatCloseMessage(gorilla.CloseNormalClosure, ""),
				time.Now().Add(time.Second),
			)
			_ = s.conn.Close()
		}
	})
}

func (s *Session) ForceClose() {
	s.stopAccepting()
	s.cancel()
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *Session) stopAccepting() {
	s.stateMu.Lock()
	s.accepting = false
	s.stateMu.Unlock()
}

func (s *Session) Shutdown(ctx context.Context) error {
	// 关闭接收通道
	s.beginShutdown()

	select {
	case <-s.writeDone:
		return nil
	case <-ctx.Done():
		s.ForceClose()
		return ctx.Err()
	}
}

func (s *Session) beginShutdown() {
	s.shutdownOnce.Do(func() {
		s.stateMu.Lock()
		s.accepting = false
		if !s.outboundClosed {
			close(s.outbound)
			s.outboundClosed = true
		}
		s.stateMu.Unlock()
	})
}

func (s *Session) Enqueue(message Message, policy AppendPolicy) error {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if !s.accepting {
		return ErrSessionClosed
	}

	select {
	case <-s.ctx.Done():
		return ErrSessionClosed
	default:
	}

	select {
	case s.outbound <- OutboundItem{Message: message, Policy: policy}:
		return nil
	case <-s.ctx.Done():
		return ErrSessionClosed
	default:
		return ErrOutboundQueueFull
	}
}

func (s *Session) UserID() string    { return s.identity.UserId }
func (s *Session) SessionID() string { return s.identity.SessionId }

func (s *Session) Platform() Platform {
	return s.identity.Platform
}

func (s *Session) SetReadLimit(size int64) {
	if size > 0 {
		s.maxReadSize = size
	}
}

func (s *Session) LastActive() time.Time {
	s.activityMu.RLock()
	idle := s.idle
	s.activityMu.RUnlock()
	return time.UnixMilli(idle)
}

func (s *Session) read(pongWait int) (*Message, error) {
	messageType, data, err := s.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if messageType != gorilla.BinaryMessage {
		return nil, fmt.Errorf("不支持的实时通道消息类型：%d", messageType)
	}

	var frame wspb.WsFrame
	if err := proto.Unmarshal(data, &frame); err != nil {
		return nil, err
	}

	s.touch()
	_ = s.conn.SetReadDeadline(time.Now().Add(time.Duration(pongWait) * time.Second))
	return &Message{Op: frame.GetOp(), Data: frame.GetData()}, nil
}

func (s *Session) PushEvent(
	op string,
	payload []byte,
) error {
	encoded, err := EncodePayload(op, payload)
	if err != nil {
		return err
	}

	return s.Enqueue(
		Message{
			Op:   op,
			Data: encoded,
		},
		AppendPolicyForEvent(op),
	)
}

func (s *Session) dispatchLoop(handler MessageHandler) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case message := <-s.inbound:
			queueWait := time.Since(message.inboundEnqueuedAt)
			queueDepth := len(s.inbound)
			if handler == nil {
				diagnostics.Logf("stage=ws_inbound_dispatch session_id=%s op=%s queue_wait_us=%d handler_us=0 queue_depth=%d queue_capacity=%d payload_bytes=%d outcome=handler_missing",
					s.SessionID(), message.Op, queueWait.Microseconds(), queueDepth, cap(s.inbound), len(message.Data))
				continue
			}
			handlerStartedAt := time.Now()
			handler(s.ctx, s, message.Op, message.Data)
			diagnostics.Logf("stage=ws_inbound_dispatch session_id=%s op=%s queue_wait_us=%d handler_us=%d queue_depth=%d queue_capacity=%d payload_bytes=%d outcome=handled",
				s.SessionID(), message.Op, queueWait.Microseconds(), time.Since(handlerStartedAt).Microseconds(), queueDepth, cap(s.inbound), len(message.Data))
		}
	}
}

func (s *Session) readLoop(pongWait int, handler MessageHandler, onClose func(*Session)) {
	defer func() {
		s.Close()
		if onClose != nil {
			onClose(s)
		}
	}()

	_ = s.conn.SetReadDeadline(time.Now().Add(time.Duration(pongWait) * time.Second))
	if s.maxReadSize > 0 {
		s.conn.SetReadLimit(s.maxReadSize)
	}
	s.conn.SetPongHandler(func(string) error {
		s.touch()
		return s.conn.SetReadDeadline(time.Now().Add(time.Duration(pongWait) * time.Second))
	})

	// 聚合读取和执行业务协程，二者通过 inbound 实现通信
	go s.dispatchLoop(handler)

	for {
		message, err := s.read(pongWait)
		if err != nil {
			return
		}
		message.inboundEnqueuedAt = time.Now()

		select {
		case s.inbound <- *message:
		case <-s.ctx.Done():
			return
		default:
			// 客户端请求处理不过来，关闭慢连接
			diagnostics.LogfAlways("stage=ws_inbound_queue session_id=%s op=%s queue_depth=%d queue_capacity=%d payload_bytes=%d outcome=full",
				s.SessionID(), message.Op, len(s.inbound), cap(s.inbound), len(message.Data))
			s.Close()
			return
		}
	}
}

func (s *Session) accumulateLoop(
	ctx context.Context,
	accumulator *BatchMessageAccumulator,
	readyQueue chan<- *MessageBatch,
) error {
	if accumulator == nil {
		return errors.New("batch message accumulator is nil")
	}

	defer close(readyQueue)

	emitBatch := func(batch *MessageBatch) error {
		if batch == nil || batch.MessageCount() == 0 {
			return nil
		}

		select {
		case readyQueue <- batch:
			return nil

		case <-ctx.Done():
			// Session 关闭了，当前批次被 cancel
			_ = batch.Cancel()
			return ctx.Err()
		}
	}

	for {
		select {
		case <-ctx.Done():
			if accumulator.activeBatch != nil {
				accumulator.activeBatch.Cancel()
			}

			return ctx.Err()

		case item, ok := <-s.outbound:
			if !ok {
				batch, err := accumulator.Flush(FlushReasonShutdown)
				if err != nil {
					return fmt.Errorf("关闭前封口消息批次失败: %w", err)
				}

				if err := emitBatch(batch); err != nil {
					return err
				}

				return nil
			}

			readyBatches, err := accumulator.Append(item.Message, item.Policy)
			if err != nil {
				return fmt.Errorf("消息加入聚合器失败: %w", err)
			}

			for _, batch := range readyBatches {
				if err := emitBatch(batch); err != nil {
					return err
				}
			}

		case <-accumulator.TimerC():
			// 一批数据到时间了，依旧封口
			batch, err := accumulator.Flush(FlushReasonLinger)
			if err != nil {
				return fmt.Errorf("linger 超时封口失败: %w", err)
			}

			if err := emitBatch(batch); err != nil {
				return err
			}
		}
	}
}

func (s *Session) sendLoop(
	ctx context.Context,
	readyQueue <-chan *MessageBatch,
	pingPeriod, writeWait int,
) error {
	if pingPeriod <= 0 {
		return errors.New("ping period must be greater than zero")
	}

	pingTicker := time.NewTicker(time.Duration(pingPeriod) * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case batch, ok := <-readyQueue:
			if !ok {
				return nil
			}

			if batch == nil || batch.MessageCount() == 0 {
				continue
			}

			if err := s.sendBatch(batch, writeWait); err != nil {
				return fmt.Errorf(
					"发送消息批次失败, batch_id=%s: %w",
					batch.Id,
					err,
				)
			}
		case <-ctx.Done():
			return ctx.Err()

		case <-pingTicker.C:
			if err := s.write(
				gorilla.PingMessage,
				nil,
				writeWait,
			); err != nil {
				return fmt.Errorf("发送 WebSocket Ping 失败: %w", err)
			}
		}
	}
}

func (s *Session) writeLoop(pingPeriod, writeWait int) {
	defer func() {
		s.Close()
		close(s.writeDone)
	}()

	err := s.writeBatchLoop(pingPeriod, writeWait)

	if err != nil &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, ErrSessionClosed) {
		log.Printf("Session 写循环退出：%v", err)
	}
}

func (s *Session) writeBatchLoop(pingPeriod, writeWait int) error {
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	accumulator := NewBatchMessageAccumulator(s.batchConfig, s.idGenerator)
	readyQueue := make(chan *MessageBatch, s.batchConfig.ReadyQueueSize)
	errCh := make(chan error, 2)

	go func() {
		errCh <- s.accumulateLoop(ctx, accumulator, readyQueue)
	}()

	go func() {
		errCh <- s.sendLoop(ctx, readyQueue, pingPeriod, writeWait)
	}()

	firstErr := <-errCh
	if firstErr != nil {
		cancel()
	}
	secondErr := <-errCh

	if isExpectedSessionError(firstErr) {
		return secondErr
	}
	return firstErr
}

func isExpectedSessionError(err error) bool {
	return err == nil ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, ErrSessionClosed)
}

func (s *Session) write(messageType int, data []byte, writeWait int) error {
	_ = s.conn.SetWriteDeadline(time.Now().Add(time.Duration(writeWait) * time.Second))
	return s.conn.WriteMessage(messageType, data)
}

func (s *Session) touch() {
	s.activityMu.Lock()
	s.idle = time.Now().UnixMilli()
	s.activityMu.Unlock()
}
