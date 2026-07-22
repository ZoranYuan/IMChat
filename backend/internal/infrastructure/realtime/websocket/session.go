package websocket

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"IM_backend/internal/infrastructure/id/snow"
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
	Op   string
	Data []byte
}

type OutboundItem struct {
	Message Message
	Policy  AppendPolicy
}

type MessageHandler func(context.Context, *Session, string, []byte)

type Session struct {
	conn      *gorilla.Conn
	ctx       context.Context
	userID    string
	sessionID string

	idGenerator  *snow.Generator
	batchConfig  MessageBatchConfig
	batchEnabled bool
	outbound     chan OutboundItem
	cancel       context.CancelFunc

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
	userID, sessionID string,
	maxBufferSize int,
	idGenerator *snow.Generator,
	batchConfig MessageBatchConfig,
	batchEnabled bool,
) *Session {
	return &Session{
		conn:         conn,
		ctx:          ctx,
		cancel:       cancel,
		idle:         time.Now().UnixMilli(),
		userID:       userID,
		sessionID:    sessionID,
		idGenerator:  idGenerator,
		batchConfig:  batchConfig.withDefaults(),
		batchEnabled: batchEnabled,
		outbound:     make(chan OutboundItem, maxBufferSize),
		writeDone:    make(chan struct{}),
		accepting:    true,
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

	payload, err := s.marshalBatch(batch)
	if err != nil {
		_ = batch.MarkFailed(err)
		return err
	}

	if err := s.write(gorilla.BinaryMessage, payload, writeWait); err != nil {
		_ = batch.MarkFailed(err)
		return err
	}

	return batch.MarkSent()
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

func (s *Session) UserID() string    { return s.userID }
func (s *Session) SessionID() string { return s.sessionID }

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

func (s *Session) readLoop(pongWait int, handler MessageHandler, onClose func(*Session)) {
	defer func() {
		s.Close()
		if onClose != nil {
			onClose(s)
		}
	}()

	_ = s.conn.SetReadDeadline(time.Now().Add(time.Duration(pongWait) * time.Second))
	s.conn.SetPongHandler(func(string) error {
		s.touch()
		return s.conn.SetReadDeadline(time.Now().Add(time.Duration(pongWait) * time.Second))
	})

	for {
		message, err := s.read(pongWait)
		if err != nil {
			return
		}
		if handler != nil {
			handler(s.ctx, s, message.Op, message.Data)
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

	var err error
	if s.batchEnabled {
		err = s.writeBatchLoop(pingPeriod, writeWait)
	} else {
		err = s.writeDirectLoop(pingPeriod, writeWait)
	}

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

func (s *Session) writeDirectLoop(pingPeriod, writeWait int) error {
	if pingPeriod <= 0 {
		return errors.New("ping period must be greater than zero")
	}

	pingTicker := time.NewTicker(time.Duration(pingPeriod) * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case item, ok := <-s.outbound:
			if !ok {
				return nil
			}
			payload, err := proto.Marshal(&wspb.WsFrame{
				Op:   item.Message.Op,
				Data: item.Message.Data,
			})
			if err != nil {
				return fmt.Errorf("序列化实时通道消息帧失败: %w", err)
			}
			if err := s.write(gorilla.BinaryMessage, payload, writeWait); err != nil {
				return err
			}
		case <-pingTicker.C:
			if err := s.write(gorilla.PingMessage, nil, writeWait); err != nil {
				return fmt.Errorf("发送 WebSocket Ping 失败: %w", err)
			}
		}
	}
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
