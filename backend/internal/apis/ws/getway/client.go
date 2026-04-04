package ws

import "github.com/gorilla/websocket"

type Client struct {
	conn      *websocket.Conn
	userId    string
	sessionId string
	send      chan Message

	close chan struct{}
}

func NewClient(conn *websocket.Conn, userId string, sessionId string, maxBufferSize int) *Client {
	c := &Client{
		conn:      conn,
		userId:    userId,
		sessionId: sessionId,
		send:      make(chan Message, maxBufferSize),
		close:     make(chan struct{}),
	}

	go c.writeHandle()
	go c.readHandle()

	return c
}

func (c *Client) Send(msg Message) error {

}

func (c *Client) Close() {
	c.close <- struct{}{}
}

func (c *Client) readHandle() {
	for {
		select {
		case msg := <-c.send:
			// 写数据
		case <-c.close:
			// TODO 关闭连接
		}
	}
}

func (*Client) writeHandle() {}
