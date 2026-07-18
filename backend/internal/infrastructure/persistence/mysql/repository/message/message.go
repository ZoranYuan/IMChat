package message

import (
	"context"
	"fmt"

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
func (r *MessageRepository) CreateNewMessage(ctx context.Context, msg *messageentity.Message) error {
	m := toMessageModel(msg)
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *MessageRepository) CreateNewMessages(ctx context.Context, msgs []*messageentity.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	models := make([]*model.Message, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			continue
		}

		models = append(models, toMessageModel(msg))
	}

	if len(models) == 0 {
		return nil
	}

	if err := r.db.WithContext(ctx).Create(&models).Error; err != nil {
		return fmt.Errorf("创建消息失败：%w", err)
	}

	return nil
}

func (r *MessageRepository) GetHistoryMessage(
	ctx context.Context,
	conversationId string,
	maxSeq int64,
	limit int,
) ([]*messageentity.Message, error) {

	var models []*model.Message

	// 只找出聊天的历史消息
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND seq < ? AND (video_id = '' OR video_id IS NULL)", conversationId, maxSeq).
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

func (r *MessageRepository) GetDanmakuByRoomVideo(
	ctx context.Context,
	conversationId string,
	videoId string,
	startTime int64,
	endTime int64,
	limit int,
) ([]*messageentity.Message, error) {
	var models []*model.Message

	query := r.db.WithContext(ctx).
		Where("conversation_id = ? AND video_id = ? AND video_time IS NOT NULL AND video_time >= ?", conversationId, videoId, startTime)
	if endTime > 0 {
		query = query.Where("video_time <= ?", endTime)
	}

	err := query.
		Order("video_time ASC").
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

func (r *MessageRepository) GetRoomVideoHistory(
	ctx context.Context,
	conversationId string,
	limit int,
) ([]*messageentity.Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	subQuery := r.db.WithContext(ctx).
		Table("messages").
		Select("video_id, MAX(seq) AS latest_seq").
		Where("conversation_id = ? AND video_id <> '' AND video_time IS NOT NULL", conversationId).
		Group("video_id").
		Order("latest_seq DESC").
		Limit(limit)

	var models []*model.Message
	err := r.db.WithContext(ctx).
		Table("messages AS m").
		Joins("JOIN (?) AS t ON m.video_id = t.video_id AND m.seq = t.latest_seq", subQuery).
		Where("m.conversation_id = ? AND m.video_id <> '' AND m.video_time IS NOT NULL", conversationId).
		Order("m.seq DESC").
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

func (r *MessageRepository) CountRoomVideoMessages(
	ctx context.Context,
	conversationId string,
	videoId string,
) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Message{}).
		Where("conversation_id = ? AND video_id = ? AND video_time IS NOT NULL", conversationId, videoId).
		Count(&count).Error
	return count, err
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
