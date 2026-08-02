package ws

type MessageReadAckReq struct {
	ConversationId string `json:"conversationId"`
	LastReadSeq    int64  `json:"lastReadSeq"`
}

type MessageReq struct {
	ClientMsgId string `json:"clientMsgId"`
	RecvId      string `json:"recvId"`
	ConvType    int    `json:"convType"`
	CType       int    `json:"cType"`
	Content     string `json:"content"`
	FileId      string `json:"fileId,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	DurationMs  *int64 `json:"durationMs,omitempty"`
	StickerId   string `json:"stickerId,omitempty"`
	PackId      string `json:"packId,omitempty"`
	VideoTime   *int64 `json:"videoTime"`
}
