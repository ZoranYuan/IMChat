package ws

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	wspb "IM_backend/internal/transport/ws/pb"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	conn      *websocket.Conn
	ctx       context.Context
	userId    string
	sessionId string

	closeOnce sync.Once

	idle              int64
	maxConnectionIdle int64

	send chan WsMessage

	cancel context.CancelFunc
	close  chan struct{}

	mu sync.RWMutex
}

func NewClient(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, userId string, sessionId string, maxBufferSize int) *Client {
	c := &Client{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
		idle:   time.Now().UnixMilli(),

		userId:    userId,
		sessionId: sessionId,
		send:      make(chan WsMessage, maxBufferSize),
		close:     make(chan struct{}),
	}

	return c
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		log.Println("连接已关闭")

		if c.conn == nil {
			return
		}

		close(c.send)
		close(c.close)

		_ = c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second),
		)

		_ = c.conn.Close()
		c.conn = nil
		c.send = nil
	})
}

func (c *Client) Write(messageType int, data []byte, writeWaitSeconds int) error {
	c.conn.SetWriteDeadline(time.Now().Add(time.Duration(writeWaitSeconds) * time.Second))
	return c.conn.WriteMessage(messageType, data)
}

func (c *Client) Read(pongWaitSeconds int) (*WsMessage, error) {
	messageType, msg, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	var wsMsg WsMessage
	if messageType != websocket.BinaryMessage {
		return nil, fmt.Errorf("unsupported websocket message type: %d", messageType)
	}

	var frame wspb.WsFrame
	if err := proto.Unmarshal(msg, &frame); err != nil {
		return nil, err
	}
	wsMsg.Op = frame.GetOp()
	wsMsg.Data = frame.GetData()

	c.mu.Lock()
	c.idle = time.Now().UnixMilli()
	c.mu.Unlock()

	c.conn.SetReadDeadline(time.Now().Add(time.Duration(pongWaitSeconds) * time.Second))
	return &wsMsg, nil
}
