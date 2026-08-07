package entity

import uconvvo "IM_backend/internal/domain/conversation/value_object"

type UserConversation struct {
	UserId         string
	ConversationId string
	LastReadSeq    int64
	IsMuted        bool
	Convtype       uconvvo.ConvType
}

func BuildUserConversation(
	userId string,
	conversationId string,
	lastReadSeq int64,
	convtype uconvvo.ConvType,
) *UserConversation {
	return &UserConversation{
		UserId:         userId,
		ConversationId: conversationId,
		LastReadSeq:    lastReadSeq,
		Convtype:       convtype,
	}
}

func (uc *UserConversation) UpdateReadSeq(lastReadSeq int64) {
	uc.LastReadSeq = lastReadSeq
}
