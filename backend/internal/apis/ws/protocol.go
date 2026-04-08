package ws

type WsMessage struct {
	Op   string `json:"op"`
	Data any    `json:"data"`
}

type MessageReqData struct {
	ConversationId string
	RecvId         string `json:"recvId"`

	ConvType int `json:"convType"` // 1 表示私聊，2 表示群聊

	CType   int    `json:"cType"`   // text, image, file...
	Content string `json:"content"` // 消息内容

	VideoTime int64 `json:"videoTime,omitempty"` // 如果不传递就是普通的聊天
}

type MessageResData struct {
	MessageId      string
	ConversationId string
	RecvId         string `json:"recvId"`

	CType   int    `json:"cType"`   // text, image, file...
	Content string `json:"content"` // 消息内容

	SendTime  int64
	VideoTime int64
}
