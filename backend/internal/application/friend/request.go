package friend

import (
	idport "IM_backend/internal/application/ports/id"
	outboxport "IM_backend/internal/application/ports/outbox"
	convcache "IM_backend/internal/application/ports/persistence/cache/conversation"
	friendcache "IM_backend/internal/application/ports/persistence/cache/friend"
	conversationrepo "IM_backend/internal/application/ports/persistence/repository/conversation"
	friendrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	friendrequestrepo "IM_backend/internal/application/ports/persistence/repository/friend"
	messagerepo "IM_backend/internal/application/ports/persistence/repository/message"
	userrepo "IM_backend/internal/application/ports/persistence/repository/user"
	txmanager "IM_backend/internal/application/ports/persistence/tx_manager"
	conversationentity "IM_backend/internal/domain/conversation/entity"
	conversationvo "IM_backend/internal/domain/conversation/value_object"
	uconvvo "IM_backend/internal/domain/conversation/value_object"
	friendentity "IM_backend/internal/domain/friend/entity"
	friendrequestentity "IM_backend/internal/domain/friend/entity"
	friendrequestvo "IM_backend/internal/domain/friend/value_object"
	friendvo "IM_backend/internal/domain/friend/value_object"
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	"IM_backend/internal/shared/protocol"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

type RequestApplication struct {
	friendRequestRepository friendrequestrepo.FriendRequestRepository
	userRepository          userrepo.UserRepository
	messageRepository       messagerepo.MessageRepository
	friendRepository        friendrepo.FriendRepository
	conversationCache       convcache.ConversationCache
	conversationRepository  conversationrepo.ConversationRepository
	userConvRepository      conversationrepo.UserConversationRepository
	friendCache             friendcache.FriendCache
	txManager               txmanager.TxManager
	idGenerator             idport.Generator
	outboxRepository        outboxport.Repository
}

func NewRequestApplication(
	friendRequestRepository friendrequestrepo.FriendRequestRepository,
	userRepository userrepo.UserRepository,
	messageRepository messagerepo.MessageRepository,
	conversationRepository conversationrepo.ConversationRepository,
	userConvRepository conversationrepo.UserConversationRepository,
	conversationCache convcache.ConversationCache,
	friendRepository friendrepo.FriendRepository,
	friendCache friendcache.FriendCache,
	txManager txmanager.TxManager,
	idGenerator idport.Generator,
	outboxRepository outboxport.Repository,
) *RequestApplication {
	return &RequestApplication{
		friendRequestRepository: friendRequestRepository,
		userRepository:          userRepository,
		messageRepository:       messageRepository,
		userConvRepository:      userConvRepository,
		conversationRepository:  conversationRepository,
		conversationCache:       conversationCache,
		friendRepository:        friendRepository,
		friendCache:             friendCache,
		txManager:               txManager,
		idGenerator:             idGenerator,
		outboxRepository:        outboxRepository,
	}
}

func (fa *RequestApplication) createNewFriendRequest(
	ctx context.Context,
	userID string,
	toUserID string,
	message string,
	repository friendrequestrepo.FriendRequestRepository,
	outboxRepository outboxport.Repository,
) (*FriendRequestDTO, error) {
	requestID, err := fa.idGenerator.Generate()
	if err != nil {
		return nil, fmt.Errorf("生成好友申请 ID 失败：%w", err)
	}

	request, err := friendrequestentity.NewFriendRequest(
		requestID,
		userID,
		toUserID,
		message,
	)
	if err != nil {
		return nil, err
	}

	record, err := repository.Create(request)
	if err != nil {
		return nil, fmt.Errorf("创建好友申请失败：%w", err)
	}

	if err := fa.createFriendRequestOutbox(ctx, record, outboxRepository); err != nil {
		return nil, fmt.Errorf("创建好友申请提醒失败：%w", err)
	}

	dto := toDTO(record)
	return &dto, nil
}

func (fa *RequestApplication) createFriendRequestOutbox(
	ctx context.Context,
	record *friendrequestentity.FriendRequest,
	repository outboxport.Repository,
) error {
	if repository == nil || record == nil {
		return nil
	}

	payload, err := json.Marshal(protocol.FriendRequestCreatedEvent{
		RequestId:  record.RequestId,
		FromUserId: record.FromUserId,
		ToUserId:   record.ToUserId,
		Message:    record.Message,
		ApplyTime:  record.ApplyTime,
	})
	if err != nil {
		return err
	}

	envelope, err := json.Marshal(protocol.Envelope{
		From:    record.FromUserId,
		To:      record.ToUserId,
		Payload: payload,
	})
	if err != nil {
		return err
	}

	return repository.Create(ctx, &outboxport.Entry{
		EventType:  protocol.EventFriendRequestCreated,
		MessageKey: record.ToUserId,
		Payload:    envelope,
	})
}

func (fa *RequestApplication) reRequest(
	ctx context.Context,
	record *friendrequestentity.FriendRequest,
	message string,
	repository friendrequestrepo.FriendRequestRepository,
	outboxRepository outboxport.Repository,
) (*FriendRequestDTO, error) {
	requestId, err := fa.idGenerator.Generate()
	if err != nil {
		return nil, fmt.Errorf("生成好友申请 ID 失败：%w", err)
	}

	if err := record.ReRequest(requestId, message); err != nil {
		return nil, err
	}

	if err := repository.ReRequest(record); err != nil {
		return nil, fmt.Errorf("更新好友申请失败：%w", err)
	}

	if err := fa.createFriendRequestOutbox(ctx, record, outboxRepository); err != nil {
		return nil, fmt.Errorf("创建好友申请提醒失败：%w", err)
	}

	dto := toDTO(record)
	return &dto, nil
}

func (fa *RequestApplication) CreateFriendRequest(
	userID string,
	toUserID string,
	message string,
) (*FriendRequestDTO, error) {
	if userID == toUserID {
		return nil, ErrSelfRequest
	}

	relation, err := fa.friendRepository.FindRelation(userID, toUserID)
	if err != nil {
		return nil, fmt.Errorf("查询好友关系失败：%w", err)
	}
	if relation != nil {
		return nil, ErrAlreadyFriends
	}

	user, err := fa.userRepository.FindByUserID(toUserID)
	if err != nil {
		return nil, fmt.Errorf("查询目标用户失败：%w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	reverse, err := fa.friendRequestRepository.FindLatestRequest(toUserID, userID)
	if err != nil {
		return nil, fmt.Errorf("查询反向好友申请失败：%w", err)
	}
	if reverse != nil && reverse.Status == friendrequestvo.Pending {
		if err := reverse.Accept(userID); err != nil {
			if errors.Is(err, friendrequestentity.ErrDuplicateRequestOperation) {
				return nil, ErrDuplicateOperation
			}
			return nil, err
		}
		if err := fa.acceptFriendRequest(reverse, userID); err != nil {
			return nil, err
		}
		dto := toDTO(reverse)
		return &dto, nil
	}

	record, err := fa.friendRequestRepository.FindLatestRequest(
		userID,
		toUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("查询最新好友申请失败：%w", err)
	}

	if record == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var result *FriendRequestDTO
		if err := fa.txManager.WithinTransaction(ctx, func(tx any) error {
			dto, err := fa.createNewFriendRequest(
				ctx,
				userID,
				toUserID,
				message,
				fa.friendRequestRepository.WithTx(tx),
				fa.outboxRepository.WithTx(tx),
			)
			if err != nil {
				return err
			}
			result = dto
			return nil
		}); err != nil {
			return nil, err
		}
		return result, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result *FriendRequestDTO
	if err := fa.txManager.WithinTransaction(ctx, func(tx any) error {
		dto, err := fa.reRequest(
			ctx,
			record,
			message,
			fa.friendRequestRepository.WithTx(tx),
			fa.outboxRepository.WithTx(tx),
		)
		if err != nil {
			return err
		}
		result = dto
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func (fa *RequestApplication) Refuse(requestId string, userId string) error {
	record, err := fa.friendRequestRepository.FindByRequestID(requestId)

	if err != nil {
		return err
	}

	if err := record.Refuse(userId); err != nil {
		return err
	}

	if err := fa.friendRequestRepository.OperateRequest(record.RequestId, int(friendrequestvo.Pending), int(record.Status)); err != nil {
		return err
	}

	return nil
}

func (fa *RequestApplication) Accept(requestId string, userId string, otherId string) error {
	record, err := fa.friendRequestRepository.FindByRequestID(requestId)

	if err != nil {
		return err
	}

	if err := record.Accept(userId); err != nil {
		if errors.Is(err, friendrequestentity.ErrDuplicateRequestOperation) {
			return ErrDuplicateOperation
		} else {
			return ErrUnknown
		}
	}

	return fa.acceptFriendRequest(record, userId)
}

func (fa *RequestApplication) acceptFriendRequest(
	record *friendrequestentity.FriendRequest,
	userID string,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 初始化元信息
	convId := conversationentity.GetConversationID(record.FromUserId, record.ToUserId, int(conversationvo.PrivateChat))

	// 创建会话
	conv := conversationentity.NewConversation(
		convId,
		record.FromUserId,
		record.ToUserId,
		int(conversationvo.PrivateChat),
		0,
		"",
	)

	toUserConv := conversationentity.BuildUserConversation(
		userID,
		convId,
		0,
		0,
		uconvvo.PrivateChat,
	)

	fromUserConv := conversationentity.BuildUserConversation(
		record.FromUserId,
		convId,
		0,
		0,
		uconvvo.PrivateChat,
	)

	messages := []*messageentity.Message{}

	// 初始化数据
	if record.Message != "" {
		greetMessageId, err := fa.idGenerator.Generate()
		if err != nil {
			return err
		}
		greetReplyMessageId, err := fa.idGenerator.Generate()
		if err != nil {
			return err
		}

		messages = append(messages, []*messageentity.Message{{
			MessageId:      greetMessageId,
			ConversationId: convId,
			SendId:         record.FromUserId,
			Seq:            1,
			Type:           messagevo.Text,
			Content:        record.Message,
			SendTime:       record.ApplyTime,
			Status:         messagevo.Normal,
		}, {
			MessageId:      greetReplyMessageId,
			ConversationId: convId,
			SendId:         record.ToUserId,
			Seq:            2,
			Type:           messagevo.Text,
			Content:        "我们已经是好友了，开始聊天吧~",
			SendTime:       time.Now().UnixMilli(),
			Status:         messagevo.Normal,
		}}...)
	} else {
		// 请求方需要主动插入请求时附带的消息
		greetMessageId, err := fa.idGenerator.Generate()
		if err != nil {
			return err
		}

		messages = append(messages, &messageentity.Message{
			MessageId:      greetMessageId,
			ConversationId: convId,
			SendId:         record.ToUserId,
			Seq:            1,
			Type:           messagevo.Text,
			Content:        "我们已经是好友了，开始聊天吧~",
			SendTime:       time.Now().UnixMilli(),
			Status:         messagevo.Normal,
		})
	}

	conv.LatestMessageId = messages[len(messages)-1].MessageId
	conv.LatestSeq = int64(len(messages))
	toUserConv.LastReadSeq = int64(len(messages))

	fromUserConv.LastReadSeq = int64(len(messages))

	if err := fa.txManager.WithinTransaction(ctx, func(tx any) error {
		if err := fa.friendRequestRepository.WithTx(tx).OperateRequest(record.RequestId, int(friendrequestvo.Pending), int(record.Status)); err != nil {
			fmt.Println("处理好友申请失败：", err)
			return ErrOperationFailed
		}

		if err := fa.friendRepository.WithTx(tx).Create([]friendentity.Friend{
			{
				UserId:       record.FromUserId,
				FriendUserId: record.ToUserId,
				Status:       friendvo.Status(friendvo.Friend),
			},
			{
				UserId:       record.ToUserId,
				FriendUserId: record.FromUserId,
				Status:       friendvo.Status(friendvo.Friend),
			},
		}); err != nil {
			log.Println("创建好友关系失败：", err)
			return ErrOperationFailed
		}

		if err := fa.conversationRepository.WithTx(tx).CreateConversation(ctx, conv); err != nil {
			return ErrCreateConvFailed
		}

		if err := fa.userConvRepository.WithTx(tx).CreateUserConversation(ctx, toUserConv); err != nil {
			return ErrCreateUserConvFailed
		}

		if err := fa.userConvRepository.WithTx(tx).CreateUserConversation(ctx, fromUserConv); err != nil {
			return ErrCreateUserConvFailed
		}

		// 初始消息必须和好友关系、会话处于同一事务，并进入 Outbox。
		if err := fa.messageRepository.WithTx(tx).CreateNewMessages(ctx, messages); err != nil {
			return err
		}
		if err := fa.createInitialMessageOutboxes(ctx, tx, record, conv, messages); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return ErrOperationFailed
	}

	if err := fa.conversationCache.RecoverConvLatestSeq(
		ctx,
		convId,
		conv.LatestSeq,
	); err != nil {
		log.Println("预热会话序列缓存失败：", err)
	}

	state := &friendcache.RelationState{Status: friendvo.Friend}
	if err := fa.friendCache.SetRelation(ctx, record.FromUserId, record.ToUserId, state); err != nil {
		log.Println("预热好友关系缓存失败：", err)
	}
	if err := fa.friendCache.SetRelation(ctx, record.ToUserId, record.FromUserId, state); err != nil {
		log.Println("预热反向好友关系缓存失败：", err)
	}
	return nil
}

func (fa *RequestApplication) createInitialMessageOutboxes(
	ctx context.Context,
	tx any,
	record *friendrequestentity.FriendRequest,
	conversation *conversationentity.Conversation,
	messages []*messageentity.Message,
) error {
	if fa.outboxRepository == nil || record == nil || conversation == nil {
		return fmt.Errorf("好友消息出箱组件未配置")
	}
	repository := fa.outboxRepository.WithTx(tx)
	for _, message := range messages {
		if message == nil {
			continue
		}
		receiverID := record.FromUserId
		if message.SendId == record.FromUserId {
			receiverID = record.ToUserId
		}
		eventPayload, err := json.Marshal(protocol.MessageEvent{
			MessageId:      message.MessageId,
			ConversationId: conversation.ConversationId,
			SendId:         message.SendId,
			RecvId:         receiverID,
			Seq:            message.Seq,
			ConvType:       protocol.PrivateChat,
			CType:          int(message.Type),
			Content:        message.Content,
			SendTime:       message.SendTime,
		})
		if err != nil {
			return err
		}
		envelope, err := json.Marshal(protocol.Envelope{
			From:    message.SendId,
			To:      receiverID,
			Payload: eventPayload,
		})
		if err != nil {
			return err
		}
		if err := repository.Create(ctx, &outboxport.Entry{
			EventType:  protocol.EventTypeSendMessage,
			MessageKey: conversation.ConversationId,
			Payload:    envelope,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (fa *RequestApplication) ListFriendRequestsByUserID(userId string) ([]FriendRequestDTO, error) {
	records, err := fa.friendRequestRepository.ListByUserID(userId)
	if err != nil {
		return nil, err
	}

	friendRequestDTOs := make([]FriendRequestDTO, 0, len(records))
	for _, r := range records {
		dto := toDTO(r)
		user, err := fa.userRepository.FindByUserID(r.FromUserId)
		if err == nil && user != nil {
			dto.FromUsername = user.UserName
			if user.NickName != "" {
				dto.FromDisplayName = user.NickName
			} else {
				dto.FromDisplayName = user.UserName
			}
		}
		friendRequestDTOs = append(friendRequestDTOs, dto)
	}

	return friendRequestDTOs, nil
}
