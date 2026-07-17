package main

import (
	"IM_backend/configs"
	fileapp "IM_backend/internal/application/file"
	friendapp "IM_backend/internal/application/friend"
	friendrequestapp "IM_backend/internal/application/friend_request"
	messageapp "IM_backend/internal/application/message"
	roomapp "IM_backend/internal/application/room"
	testdataapp "IM_backend/internal/application/testdata"
	userapp "IM_backend/internal/application/user"
	mq "IM_backend/internal/infrastructure/messaging"
	"IM_backend/internal/infrastructure/messaging/client/kafka"
	"IM_backend/internal/infrastructure/persistence"
	"IM_backend/internal/infrastructure/persistence/mysql"
	filemysql "IM_backend/internal/infrastructure/persistence/mysql/repository/file"
	friendmysql "IM_backend/internal/infrastructure/persistence/mysql/repository/friend"
	friendrequestmysql "IM_backend/internal/infrastructure/persistence/mysql/repository/friend_request"
	messagemysql "IM_backend/internal/infrastructure/persistence/mysql/repository/message"
	roommysql "IM_backend/internal/infrastructure/persistence/mysql/repository/room"
	roomusermysql "IM_backend/internal/infrastructure/persistence/mysql/repository/room_user"
	usermysql "IM_backend/internal/infrastructure/persistence/mysql/repository/user"
	"IM_backend/internal/infrastructure/persistence/redis"
	authredis "IM_backend/internal/infrastructure/persistence/redis/cache/auth"
	conversationredis "IM_backend/internal/infrastructure/persistence/redis/cache/conversation"
	fileredis "IM_backend/internal/infrastructure/persistence/redis/cache/file"
	friendredis "IM_backend/internal/infrastructure/persistence/redis/cache/friend"
	messageredis "IM_backend/internal/infrastructure/persistence/redis/cache/message"
	roomredis "IM_backend/internal/infrastructure/persistence/redis/cache/room"
	"IM_backend/internal/infrastructure/ratelimit"
	realtimews "IM_backend/internal/infrastructure/realtime/websocket"
	authsvc "IM_backend/internal/infrastructure/security/auth"
	minioobj "IM_backend/internal/infrastructure/storage/minio"
	"IM_backend/internal/shared/protocol"
	httpapi "IM_backend/internal/transport/http"
	filehttp "IM_backend/internal/transport/http/file"
	friendhttp "IM_backend/internal/transport/http/friend"
	friendrequesthttp "IM_backend/internal/transport/http/friend_request"
	messagehttp "IM_backend/internal/transport/http/message"
	"IM_backend/internal/transport/http/middleware"
	roomhttp "IM_backend/internal/transport/http/room"
	testdatahttp "IM_backend/internal/transport/http/testdata"
	userhttp "IM_backend/internal/transport/http/user"
	"IM_backend/internal/transport/ws"
	"context"
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
	r.Use(middleware.ErrorLoggerMiddleware())

	ctx := context.TODO()

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/workspace/IM/backend/configs/config.yaml"
	}
	cfg := configs.LoadConfig(configPath)

	if cfg.App.Env == "development" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	redisClient := redis.InitRedis(cfg.Database.Redis.DSN)
	db := mysql.InitMysql(cfg.Database.MySQL.DSN)
	defer func() {
		db = nil
	}()

	txManager := persistence.NewGormTxManager(db)

	realtimeGateway := realtimews.NewGateway()
	realtimeGateway.KeepAlive(ctx, cfg.WebSocket.TimerInterval, cfg.WebSocket.PongWaitSeconds)

	authCache := authredis.NewAuthCache(redisClient)
	roomCache := roomredis.NewRoomCache(redisClient)
	fileCache := fileredis.NewFileCache(redisClient)
	friendCache := friendredis.NewFriendCache(redisClient)
	messageCache := messageredis.NewMessageCache(redisClient)

	conversationCache := conversationredis.NewConversationCache(redisClient)

	kafkaClient, err := kafka.NewClient(cfg.Kafka)

	if err != nil {
		log.Fatalln("failed to connect kafka, ", err)
	}

	authService := authsvc.NewAuthService(cfg)

	objectStorage, err := minioobj.NewObjectStorage(ctx, cfg.Storage.MinIO)
	if err != nil {
		log.Fatalln("failed to connect minio, ", err)
	}

	// 构造依赖
	fileRepository := filemysql.NewFileRepository(db)
	fileApplication := fileapp.NewApplication(cfg, fileRepository, fileCache, objectStorage)
	fileHandle := filehttp.NewHandle(fileApplication)

	messageRepository := messagemysql.NewMessageRepository(db)
	messageImageRepository := messagemysql.NewMessageImageRepository(db)
	messageFileRepository := messagemysql.NewMessageFileRepository(db)
	messageStickerRepository := messagemysql.NewMessageStickerRepository(db)
	messageVideoRepository := messagemysql.NewMessageVideoRepository(db)
	messageOutboxRepository := messagemysql.NewMessageOutboxRepository(db, int(cfg.App.MachineID))
	conversationRepository := messagemysql.NewConversationRepository(db)
	userConversationRepository := messagemysql.NewUserConversationRepository(db)

	userRepository := usermysql.NewUserRepository(db)
	userApp := userapp.NewUserApplication(userRepository, cfg, authCache, authService)
	userHandle := userhttp.NewUserHandle(userApp)

	friendRepository := friendmysql.NewFriendRepository(db)
	friendRequestRepository := friendrequestmysql.NewFriendRequestRepository(db)
	friendRequestApp := friendrequestapp.NewFriendApplication(
		friendRequestRepository,
		userRepository,
		cfg,
		friendRepository,
		friendCache,
		txManager,
	)
	friendRequestHandle := friendrequesthttp.NewFriendRequestHandle(friendRequestApp)

	friendApp := friendapp.NewFriendApplication(friendRepository, userRepository)
	friendHandle := friendhttp.NewFriendHandle(friendApp)

	roomUserRepository := roomusermysql.NewRoomUserRepository(db)
	roomRepository := roommysql.NewRoomRepository(db)
	roomApp := roomapp.NewRoomApplication(
		roomRepository,
		roomUserRepository,
		userConversationRepository,
		conversationRepository,
		cfg,
		roomCache,
		roomCache,
		txManager,
	)
	roomHandle := roomhttp.NewRoomHandle(roomApp)

	messagePushHandler := mq.NewMessagePushHandler(
		realtimeGateway,
		roomRepository,
		roomUserRepository,
		userConversationRepository,
		roomCache,
	)
	readAckHandler := mq.NewReadAckHandler(realtimeGateway)
	conversationSyncHandler := mq.NewConversationSyncHandler(userConversationRepository)
	consumerRouter := mq.NewConsumerRouter(map[string]mq.ConsumerHandler{
		protocol.EventTypeMessage:         messagePushHandler,
		protocol.EventMessageReadAck:      readAckHandler,
		protocol.EventConversationSyncSeq: conversationSyncHandler,
	})

	messageConsumer := kafka.NewConsumerGroup(kafkaClient, []string{
		string(protocol.EventMessageReadAck),
		string(protocol.EventTypeMessage),
		string(protocol.EventConversationSyncSeq),
	},
		consumerRouter,
	)

	go func() {
		if err := messageConsumer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("kafka consumer stopped: %v", err)
		}
	}()

	messageProducer := kafka.NewProducer(kafkaClient, "msg")

	dispatcher := ws.NewDispatcher()
	taskManager := mq.NewTaskManager(messageProducer)
	readAckOutboxWorker := mq.NewReadAckOutboxWorker(txManager, messageOutboxRepository, taskManager)

	go readAckOutboxWorker.Start(ctx)

	messageApplication := messageapp.NewMessageApplication(
		cfg,
		conversationCache,
		friendCache,
		messageCache,
		roomCache,
		txManager,
		userConversationRepository,
		conversationRepository,
		messageOutboxRepository,
		friendRepository,
		fileRepository,
		messageImageRepository,
		messageFileRepository,
		messageStickerRepository,
		messageVideoRepository,
		userRepository,
		messageRepository,
		roomUserRepository,
		roomRepository,
	)
	messageHandle := messagehttp.NewMessageHandle(messageApplication)

	wsHandle := ws.NewWSHandler(
		messageApplication,
		cfg,
		dispatcher,
		realtimeGateway,
	)

	testdataApplication := testdataapp.NewBootstrapApplication(
		cfg,
		db,
		redisClient,
		userRepository,
		friendRepository,
		messageRepository,
		conversationRepository,
		userConversationRepository,
		roomRepository,
		roomUserRepository,
		roomApp,
		messageApplication,
	)
	testdataHandle := testdatahttp.NewHandle(testdataApplication)

	// 注册中间件
	authMiddle := middleware.NewAuthMiddleware(cfg, authCache)

	limiter := ratelimit.NewRedisLimit(redisClient, "rate:limit")
	limiterMiddleware := middleware.NewLimitMiddleware(limiter, true)
	wsHandle.SetLimiter(limiter)

	// 注册路由
	apiGroup := r.Group("/api/v1")
	httpapi.RegisterFriendRequestRouter(apiGroup, friendRequestHandle, authMiddle)
	httpapi.RegisterUserRouter(apiGroup, userHandle, authMiddle, limiterMiddleware)
	httpapi.RegisterFriendRouter(apiGroup, friendHandle, authMiddle)
	httpapi.RegisterRoomRouter(apiGroup, roomHandle, authMiddle)
	httpapi.RegisterMessagesRouter(apiGroup, messageHandle, authMiddle)
	httpapi.RegisterFileRouter(apiGroup, fileHandle, authMiddle)
	if cfg.App.Env == "development" {
		httpapi.RegisterTestDataRouter(apiGroup.Group("/dev"), testdataHandle)
	}

	ws.RegisterWSRouter(apiGroup, wsHandle, authMiddle)

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

	log.Println("start the server....")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to run server", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	// 关闭服务
	log.Println("finish the server....")

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("failed to finish the server", err)
	}
}
