package ws

import "encoding/json"

type WsMessage struct {
	Op   string          `json:"op"`
	Data json.RawMessage `json:"data"`
}

type SendMessageReq struct {
	RecvId   string `json:"recvId"`
	Convtype int    `json:"convType"` // 0 表示私聊，1 表示群聊

	CType     int    `json:"cType,omitempty"`     // text, image, file...
	Content   string `json:"content"`             // 消息内容
	VideoTime int64  `json:"videoTime,omitempty"` // 如果不传递就是普通的聊天
}
