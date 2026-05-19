package message

import (
	"context"

	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	messageentity "IM_backend/internal/domain/message/entity"
	"IM_backend/internal/infrastructure/persistence/mysql/model"

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) WithTx(tx any) messagerepo.MessageRepository {
	return &MessageRepository{
		db: tx.(*gorm.DB),
	}
}

// 保存消息
func (r *MessageRepository) Save(ctx context.Context, msg *messageentity.Message) error {
	m := toMessageModel(msg)
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *MessageRepository) GetHistoryMessage(
	ctx context.Context,
	conversationId string,
	maxSeq int64,
	limit int,
) ([]*messageentity.Message, error) {

	var models []*model.Message

	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND seq < ?", conversationId, maxSeq).
		Order("seq DESC").
		Limit(limit).
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*messageentity.Message, 0, len(models))
	for _, m := range models {
		result = append(result, toMessageDomain(m))
	}

	return result, nil
}

func (r *MessageRepository) GetMessagesBySendTime(
	ctx context.Context,
	conversationId string,
	startTime int64,
	endTime int64,
	limit int,
) ([]*messageentity.Message, error) {
	var models []*model.Message

	query := r.db.WithContext(ctx).
		Where("conversation_id = ? AND send_time >= ?", conversationId, startTime)
	if endTime > 0 {
		query = query.Where("send_time <= ?", endTime)
	}

	err := query.
		Order("send_time ASC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}

	result := make([]*messageentity.Message, 0, len(models))
	for _, m := range models {
		result = append(result, toMessageDomain(m))
	}

	return result, nil
}

func (r *MessageRepository) GetLatestMessagesByConversationIDs(
	ctx context.Context,
	conversationIDs []string,
) ([]*messageentity.Message, error) {

	var msgs []*model.Message

	err := r.db.WithContext(ctx).
		Raw(`
			select m.* 
			from messages m
			join conversations c
			on m.message_id = c.latest_message_id
			where c.conversation_id in (?)
        `, conversationIDs).
		Find(&msgs).Error

	var domains []*messageentity.Message

	for _, m := range msgs {
		domains = append(domains, toMessageDomain(m))
	}

	return domains, err
}
