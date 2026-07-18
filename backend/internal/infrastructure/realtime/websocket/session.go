package websocket

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	wspb "IM_backend/internal/transport/ws/pb"

	gorilla "github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

var (
	ErrSessionClosed     = errors.New("WebSocket 会话已关闭")
	ErrOutboundQueueFull = errors.New("WebSocket 发送队列已满")
)

type Message struct {
	Op   string
	Data []byte
}

type MessageHandler func(context.Context, *Session, string, []byte)

// Session owns one websocket connection and its read/write pumps.
type Session struct {
	conn      *gorilla.Conn
	ctx       context.Context
	userID    string
	sessionID string

	closeOnce sync.Once
	idle      int64
	outbound  chan Message
	cancel    context.CancelFunc
	mu        sync.RWMutex
}

func NewSession(ctx context.Context, cancel context.CancelFunc, conn *gorilla.Conn, userID, sessionID string, maxBufferSize int) *Session {
	return &Session{
		conn:      conn,
		ctx:       ctx,
		cancel:    cancel,
		idle:      time.Now().UnixMilli(),
		userID:    userID,
		sessionID: sessionID,
		outbound:  make(chan Message, maxBufferSize),
	}
}

func (s *Session) Start(pongWait, pingPeriod, writeWait int, handler MessageHandler, onClose func(*Session)) {
	go s.readLoop(pongWait, handler, onClose)
	go s.writeLoop(pingPeriod, writeWait)
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.cancel()
		_ = s.conn.WriteControl(
			gorilla.CloseMessage,
			gorilla.FormatCloseMessage(gorilla.CloseNormalClosure, ""),
			time.Now().Add(time.Second),
		)
		_ = s.conn.Close()
	})
}

func (s *Session) Enqueue(message Message) error {
	select {
	case <-s.ctx.Done():
		return ErrSessionClosed
	default:
	}

	select {
	case s.outbound <- message:
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
	s.mu.RLock()
	idle := s.idle
	s.mu.RUnlock()
	return time.UnixMilli(idle)
}

func (s *Session) read(pongWait int) (*Message, error) {
	messageType, data, err := s.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if messageType != gorilla.BinaryMessage {
		return nil, fmt.Errorf("不支持的 WebSocket 消息类型：%d", messageType)
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

func (s *Session) writeLoop(pingPeriod, writeWait int) {
	ticker := time.NewTicker(time.Duration(pingPeriod) * time.Second)
	defer func() {
		ticker.Stop()
		s.Close()
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case message := <-s.outbound:
			payload, err := proto.Marshal(&wspb.WsFrame{Op: message.Op, Data: message.Data})
			if err != nil {
				log.Printf("序列化 WebSocket 帧失败：%v", err)
				return
			}
			if err := s.write(gorilla.BinaryMessage, payload, writeWait); err != nil {
				return
			}
		case <-ticker.C:
			if err := s.write(gorilla.PingMessage, nil, writeWait); err != nil {
				return
			}
		}
	}
}

func (s *Session) write(messageType int, data []byte, writeWait int) error {
	_ = s.conn.SetWriteDeadline(time.Now().Add(time.Duration(writeWait) * time.Second))
	return s.conn.WriteMessage(messageType, data)
}

func (s *Session) touch() {
	s.mu.Lock()
	s.idle = time.Now().UnixMilli()
	s.mu.Unlock()
}
