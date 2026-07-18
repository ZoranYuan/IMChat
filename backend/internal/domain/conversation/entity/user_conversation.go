package entity

type UserConversation struct {
	UserId         string
	ConversationId string
	LastReadSeq    int64
	LatestSyncSeq  int64
	IsMuted        bool
}

func BuildUserConversation(
	userId string,
	conversationId string,
	lastReadSeq int64,
	latestSyncSeq int64,
) *UserConversation {
	return &UserConversation{
		UserId:         userId,
		ConversationId: conversationId,
		LastReadSeq:    lastReadSeq,
		LatestSyncSeq:  latestSyncSeq,
	}
}

func (uc *UserConversation) UpdateSyncSeq(latestSyncSeq int64) {
	uc.LatestSyncSeq = latestSyncSeq
}

func (uc *UserConversation) UpdateReadSeq(lastReadSeq int64) {
	uc.LastReadSeq = lastReadSeq
}
