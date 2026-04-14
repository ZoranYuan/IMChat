package message_entity

type UserConversation struct {
	UserId         string
	ConversationId string
	LastMessageId  string
	LastReadSeq    int64
	LatestSyncSeq  int64
	IsMuted        bool
}

func BuildUserConversation(
	userId,
	conversationId string,
	messageId string,
	lastReadSeq int64,
	latestSyncSeq int64,
) *UserConversation {
	return &UserConversation{
		UserId:         userId,
		ConversationId: conversationId,
		LastMessageId:  messageId,
		LastReadSeq:    lastReadSeq,
		LatestSyncSeq:  latestSyncSeq,
		IsMuted:        false,
	}
}

func (uc *UserConversation) SyncReadSeq(
	latestSyncSeq int64,
) {
	uc.LatestSyncSeq = latestSyncSeq
}

func (uc *UserConversation) UpdateReadSeq(
	lastReadSeq int64,
) {
	uc.LastReadSeq = lastReadSeq
}
