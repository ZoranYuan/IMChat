package ws

import "encoding/json"

type WsMessage struct {
	Op   string          `json:"op"`
	Data json.RawMessage `json:"data"`
}

type MessageReqData struct {
	ClientMsgId string `json:"clientMsgId"`
	RecvId      string `json:"recvId"`
	ConvType    int    `json:"convType"`
	CType       int    `json:"cType"`
	Content     string `json:"content"`
	VideoTime   *int64 `json:"videoTime"`
}
