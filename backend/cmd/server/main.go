package main

import (
	"IM_backend/configs"
	apis "IM_backend/internal/apis/https"
	https_friend "IM_backend/internal/apis/https/friend"
	https_friend_request "IM_backend/internal/apis/https/friend_request"
	https_message "IM_backend/internal/apis/https/message"
	"IM_backend/internal/apis/https/middleware"
	https_room "IM_backend/internal/apis/https/room"
	https_user "IM_backend/internal/apis/https/user"
	"IM_backend/internal/apis/ws"
	application_friend "IM_backend/internal/applications/friend"
	application_friend_request "IM_backend/internal/applications/friend_request"
	application_message "IM_backend/internal/applications/message"
	application_room "IM_backend/internal/applications/room"
	application_user "IM_backend/internal/applications/user"
	"IM_backend/internal/infrastructure/database/mysql"
	friend_repository "IM_backend/internal/infrastructure/database/mysql/repository/friend"
	friend_request_repository "IM_backend/internal/infrastructure/database/mysql/repository/friend_request"
	message_repository "IM_backend/internal/infrastructure/database/mysql/repository/message"
	room_repository "IM_backend/internal/infrastructure/database/mysql/repository/room"
	room_user_repository "IM_backend/internal/infrastructure/database/mysql/repository/room_user"
	user_repository "IM_backend/internal/infrastructure/database/mysql/repository/user"
	"IM_backend/internal/infrastructure/database/redis"
	auth_cache "IM_backend/internal/infrastructure/database/redis/cache/auth"
	conversation_cache "IM_backend/internal/infrastructure/database/redis/cache/conversation"
	message_cache "IM_backend/internal/infrastructure/database/redis/cache/message"
	room_cache "IM_backend/internal/infrastructure/database/redis/cache/room"
	"IM_backend/internal/infrastructure/mq"
	"IM_backend/internal/infrastructure/mq/client/kafka"
	"IM_backend/internal/infrastructure/persistence"
	"IM_backend/internal/protocol"
	service_auth "IM_backend/internal/service/auth"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	ctx := context.TODO()

	cfg := configs.LoadConfig("/workspace/IM/backend/configs/config.yaml")

	if cfg.App.Env == "development" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	redis := redis.InitRedis(cfg.Database.Redis.DSN)
	db := mysql.InitMysql(cfg.Database.MySQL.DSN)
	defer func() {
		db = nil
	}()

	txManager := persistence.NewGormTxManager(db)

	getWay := ws.NewGetWay()
	go getWay.KeepAlive(cfg.WebSocket.TimerInterval, cfg.WebSocket.PongWaitSeconds)

	authCache := auth_cache.NewAuthCache(redis)
	roomCache := room_cache.NewRoomCache(redis)

	conversationCache := conversation_cache.NewConversationCache(redis)
	messageCache := message_cache.NewMessageCache(redis)

	kafkaClient, err := kafka.NewClient(cfg.Kafka)

	if err != nil {
		log.Fatalln("failed to connect kafka, ", err)
	}

	authService := service_auth.NewAuthService(cfg)

	// 构造依赖
	messageRepository := message_repository.NewMessageRepository(db)
	conversationRepository := message_repository.NewConversationRepository(db)
	userConversationRepository := message_repository.NewUserConversationRepository(db)

	userRepository := user_repository.NewUserRepository(db)
	userApp := application_user.NewUserApplication(userRepository, cfg, authCache, authService)
	userHandle := https_user.NewUserHandle(userApp)

	friendRequestRepository := friend_repository.NewFriendRepository(db)

	friendRequestRepositoy := friend_request_repository.NewFriendRequestRepository(db)
	friendRequestApp := application_friend_request.NewFriendApplication(friendRequestRepositoy, userRepository, cfg, friendRequestRepository, txManager)
	friendRequestHandle := https_friend_request.NewFriendRequestHandle(friendRequestApp)

	friendRepository := friend_repository.NewFriendRepository(db)
	friendApp := application_friend.NewFriendApplication(friendRepository, userRepository)
	friendHandle := https_friend.NewFriendHandle(friendApp)

	roomUserRepository := room_user_repository.NewRoomUserRepository(db)
	roomRepository := room_repository.NewRoomRepository(db)
	roomApp := application_room.NewRoomApplication(
		roomRepository,
		roomUserRepository,
		userConversationRepository,
		conversationRepository,
		cfg,
		roomCache,
		messageCache,
		conversationCache,
		txManager,
	)
	roomHandle := https_room.NewRoomHandle(roomApp)

	groupHandler := kafka.NewGroupHandler(getWay, userConversationRepository)

	messageConsumer := kafka.NewConsumer(kafkaClient, []string{
		string(protocol.EventTypeHistoryMessageReadAck),
		string(protocol.EventTypeMessage),
		string(protocol.EventTypeMsgAck),
	},
		fmt.Sprintf("machine-%d-group", cfg.App.MachineID),
		groupHandler,
	)

	go messageConsumer.Start(ctx)

	messageProducer := kafka.NewProducer(kafkaClient, "msg")

	dispatcher := ws.NewDispatcher()
	taskManager := mq.NewTaskManager(messageProducer, conversationCache)

	messageApplication := application_message.NewMessageApplication(
		cfg,
		messageCache,
		conversationCache,
		txManager,
		taskManager,
		userConversationRepository,
		conversationRepository,
		messageRepository,
	)
	messageHandle := https_message.NewMessageHandle(messageApplication)

	wsHandle := ws.NewWshandler(
		messageApplication,
		cfg,
		dispatcher,
		getWay,
	)

	// 注册中间件
	authMiddle := middleware.NewAuthMiddleware(cfg, authCache)

	// 注册路由
	apiGroup := r.Group("/api/v1")
	apis.RegisterFriendRequestRouter(apiGroup, friendRequestHandle, authMiddle)
	apis.RegisterUserRouter(apiGroup, userHandle, authMiddle)
	apis.RegisterFriendRouter(apiGroup, friendHandle, authMiddle)
	apis.RegisterRoomRouter(apiGroup, roomHandle, authMiddle)
	apis.RegisterMessagesRouter(apiGroup, messageHandle, authMiddle)

	ws.RegisterWsRouter(apiGroup, wsHandle, authMiddle)

	srv := &http.Server{
		Addr:         cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		// 关键：最大连接数限制
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	defer srv.Close()

	log.Println("start the serve....")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to run serve", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	// 关闭服务
	log.Println("finish the serve....")

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("failed to finish the serve", err)
	}
}
