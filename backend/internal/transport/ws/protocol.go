package ws

import "encoding/json"

type WsMessage struct {
	Op   string          `json:"op"`
	Data json.RawMessage `json:"data"`
}

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

type WatchVideoControlReq struct {
	RoomId       string  `json:"roomId"`
	Action       string  `json:"action"`
	VideoId      string  `json:"videoId"`
	VideoURL     string  `json:"videoUrl"`
	PositionMs   int64   `json:"positionMs"`
	DeltaMs      int64   `json:"deltaMs"`
	DurationMs   int64   `json:"durationMs"`
	PlaybackRate float64 `json:"playbackRate"`
	ClientTimeMs int64   `json:"clientTimeMs"`
}

type WatchVideoState struct {
	RoomId       string  `json:"roomId"`
	Action       string  `json:"action"`
	VideoId      string  `json:"videoId"`
	VideoURL     string  `json:"videoUrl"`
	PositionMs   int64   `json:"positionMs"`
	DeltaMs      int64   `json:"deltaMs"`
	DurationMs   int64   `json:"durationMs"`
	PlaybackRate float64 `json:"playbackRate"`
	IsPlaying    bool    `json:"isPlaying"`
	UpdatedBy    string  `json:"updatedBy"`
	UpdatedAtMs  int64   `json:"updatedAtMs"`
	ClientTimeMs int64   `json:"clientTimeMs"`
}
