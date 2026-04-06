package ws

import (
	"context"
	"encoding/json"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn      *websocket.Conn
	ctx       context.Context
	userId    string
	sessionId string
	send      chan WsMessage
	cancel    context.CancelFunc

	close chan struct{}
}

func NewClient(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, userId string, sessionId string, maxBufferSize int) *Client {
	c := &Client{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,

		userId:    userId,
		sessionId: sessionId,
		send:      make(chan WsMessage, maxBufferSize),
		close:     make(chan struct{}),
	}

	return c
}

func (c *Client) Close() {
	c.close <- struct{}{}
	c.cancel()
}

func (c *Client) Read() (*WsMessage, error) {
	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	var wsMsg WsMessage
	if err := json.Unmarshal(msg, &wsMsg); err != nil {
		return nil, err
	}

	return &wsMsg, nil
}
