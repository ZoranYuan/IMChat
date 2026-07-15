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
	MediaURL    string `json:"mediaUrl,omitempty"`
	ThumbURL    string `json:"thumbUrl,omitempty"`
	FileId      string `json:"fileId,omitempty"`
	ThumbFileId string `json:"thumbFileId,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	FileSize    int64  `json:"fileSize,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	DurationMs  *int64 `json:"durationMs,omitempty"`
	StickerId   string `json:"stickerId,omitempty"`
	PackId      string `json:"packId,omitempty"`
	VideoTime   *int64 `json:"videoTime"`
}
