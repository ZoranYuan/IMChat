package ws

// 没读取到一个消息都是一个 Message 对象
type Message struct {
	content []byte
}

func NewMessage(content []byte) *Message {
	return &Message{
		content: content,
	}
}
