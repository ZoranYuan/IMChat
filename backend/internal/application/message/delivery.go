package message

import (
	conversationport "IM_backend/internal/application/ports/conversation"
	roomcache "IM_backend/internal/application/ports/persistence/cache/room"
	roomrepo "IM_backend/internal/application/ports/persistence/repository/room"
	realtimeport "IM_backend/internal/application/ports/realtime"
	roomvo "IM_backend/internal/domain/room/value_object"
	"IM_backend/internal/shared/protocol"
	"context"
	"log"

	"golang.org/x/sync/singleflight"
)

type DeliveryCommand struct {
	EventType      string
	ConversationId string
	FromUserId     string
	ToUserId       string
	Payload        []byte
	Message        protocol.MessageEvent
}

type DeliveryApplication struct {
	delivery                 realtimeport.Delivery
	roomMemberCache          roomcache.RoomMemberCache
	roomRepository           roomrepo.RoomRepository
	roomUserRepository       roomrepo.RoomUserRepository
	conversationSynchronizer conversationport.Synchronizer
	singleflight             singleflight.Group
}

func NewDeliveryApplication(
	delivery realtimeport.Delivery,
	roomRepository roomrepo.RoomRepository,
	roomUserRepository roomrepo.RoomUserRepository,
	conversationSynchronizer conversationport.Synchronizer,
	roomMemberCache roomcache.RoomMemberCache,
) *DeliveryApplication {
	return &DeliveryApplication{
		delivery:                 delivery,
		roomMemberCache:          roomMemberCache,
		roomRepository:           roomRepository,
		roomUserRepository:       roomUserRepository,
		conversationSynchronizer: conversationSynchronizer,
	}
}

func (application *DeliveryApplication) DeliverMessage(ctx context.Context, command DeliveryCommand) error {
	switch command.Message.ConvType {
	case protocol.PrivateChat:
		if err := application.syncSequences(ctx, command.ConversationId, command.Message.Seq, []string{command.ToUserId}); err != nil {
			return err
		}
		if err := application.delivery.DeliverToUser(command.EventType, command.ToUserId, command.Payload); err != nil {
			return err
		}
		return nil

	case protocol.RoomChat:
		members, err := application.roomMembers(ctx, command.ConversationId)
		if err != nil {
			return err
		}
		recipients := make([]string, 0, len(members))
		for _, userId := range members {
			if userId == command.FromUserId {
				continue
			}
			recipients = append(recipients, userId)
		}
		if err := application.syncSequences(ctx, command.ConversationId, command.Message.Seq, recipients); err != nil {
			return err
		}
		for _, userId := range recipients {
			if err := application.delivery.DeliverToUser(command.EventType, userId, command.Payload); err != nil {
				return err
			}
		}
		return nil

	default:
		return ErrUnknownConversationType
	}
}

func (application *DeliveryApplication) DeliverReadNotification(
	_ context.Context,
	event protocol.MessageReadAckEvent,
	payload []byte,
) error {
	if event.SenderId == "" {
		return nil
	}
	return application.delivery.DeliverToUser(protocol.EventReadMessageNotify, event.SenderId, payload)
}

func (application *DeliveryApplication) syncSequences(
	ctx context.Context,
	conversationId string,
	latestSeq int64,
	userIds []string,
) error {
	items := make([]conversationport.SyncItem, 0, len(userIds))
	for _, userId := range userIds {
		items = append(items, conversationport.SyncItem{
			UserId:         userId,
			ConversationId: conversationId,
			LatestSeq:      latestSeq,
		})
	}
	return application.conversationSynchronizer.SyncLatestSequences(ctx, items)
}

func (application *DeliveryApplication) roomMembers(ctx context.Context, roomId string) ([]string, error) {
	members, cached, err := application.roomMemberCache.GetMemberIDs(ctx, roomId)
	if err == nil && cached {
		return members, nil
	}

	value, err, _ := application.singleflight.Do(roomId, func() (any, error) {
		members, cached, err := application.roomMemberCache.GetMemberIDs(ctx, roomId)
		if err == nil && cached {
			return members, nil
		}
		if _, err := application.roomRepository.FindActiveRoom(roomId, int(roomvo.Activate)); err != nil {
			return nil, err
		}
		members, err = application.roomUserRepository.ListActiveUserIDs(roomId)
		if err != nil {
			return nil, err
		}
		if err := application.roomMemberCache.SetMemberIDs(ctx, roomId, members); err != nil {
			log.Printf("警告：房间 %s 的成员缓存预热失败：%v", roomId, err)
		}
		return members, nil
	})
	if err != nil {
		return nil, err
	}
	return value.([]string), nil
}
