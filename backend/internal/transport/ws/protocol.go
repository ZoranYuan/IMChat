package ws

type MessageReadAckReq struct {
	MessageId string `json:"messageId"`
}

type MessageReq struct {
	ClientMsgId string `json:"clientMsgId"`
	RecvId      string `json:"recvId"`
	ConvType    int    `json:"convType"`
	CType       int    `json:"cType"`
	Content     string `json:"content"`
	FileId      string `json:"fileId,omitempty"`
	StickerId   string `json:"stickerId,omitempty"`
	PackId      string `json:"packId,omitempty"`
	VideoTime   *int64 `json:"videoTime"`
}
