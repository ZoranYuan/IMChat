package application_message

type MessageAppeDTO struct {
	ClientMsgId string
	SendId      string
	SessionId   string // 存储 UserId 和 SessionId 的映射
	RecvId      string
	ConvType    int // 聊天的方式
	Ctype       int // 消息的内容
	Content     string
	VideoTime   int64
}
