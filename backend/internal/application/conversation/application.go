package conversation

import (
	conversationrepo "IM_backend/internal/application/ports/persistence/repository/conversation"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	"context"
)

type SyncSeq struct {
	UserId         string
	ConversationId string
	LatestSeq      int64
}

type Application struct {
	userConversationRepository conversationrepo.UserConversationRepository
}

func NewApplication(userConversationRepository conversationrepo.UserConversationRepository) *Application {
	return &Application{userConversationRepository: userConversationRepository}
}

func (application *Application) SyncLatestSequences(ctx context.Context, items []SyncSeq) error {
	conversations := make([]*conversationentity.UserConversation, 0, len(items))
	for _, item := range items {
		if item.UserId == "" || item.ConversationId == "" {
			continue
		}

		conversations = append(conversations, conversationentity.BuildUserConversation(
			item.UserId,
			item.ConversationId,
			0,
			item.LatestSeq,
		))
	}

	if len(conversations) == 0 {
		return nil
	}
	return application.userConversationRepository.BatchUpdateSyncSeq(ctx, conversations)
}
