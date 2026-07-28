package main

import (
	"IM_backend/configs"
	conversationapp "IM_backend/internal/application/conversation"
	fileapp "IM_backend/internal/application/file"
	friendapp "IM_backend/internal/application/friend"
	messageapp "IM_backend/internal/application/message"
	eventbus "IM_backend/internal/application/ports/eventbus"
	roomapp "IM_backend/internal/application/room"
	userapp "IM_backend/internal/application/user"
	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/infrastructure/mq/kafka"
	outboxinfra "IM_backend/internal/infrastructure/outbox"
	"IM_backend/internal/infrastructure/persistence"
	"IM_backend/internal/infrastructure/persistence/mysql"
	conversationmysql "IM_backend/internal/infrastructure/persistence/mysql/repository/conversation"
	filemysql "IM_backend/internal/infrastructure/persistence/mysql/repository/file"
	friendmysql "IM_backend/internal/infrastructure/persistence/mysql/repository/friend"
	messagemysql "IM_backend/internal/infrastructure/persistence/mysql/repository/message"
	outboxmysql "IM_backend/internal/infrastructure/persistence/mysql/repository/outbox"
	roommysql "IM_backend/internal/infrastructure/persistence/mysql/repository/room"
	roomusermysql "IM_backend/internal/infrastructure/persistence/mysql/repository/room_user"
	usermysql "IM_backend/internal/infrastructure/persistence/mysql/repository/user"
	"IM_backend/internal/infrastructure/persistence/redis"
	authcache "IM_backend/internal/infrastructure/persistence/redis/cache/auth"
	conversationcache "IM_backend/internal/infrastructure/persistence/redis/cache/conversation"
	filecache "IM_backend/internal/infrastructure/persistence/redis/cache/file"
	friendcache "IM_backend/internal/infrastructure/persistence/redis/cache/friend"
	messagecache "IM_backend/internal/infrastructure/persistence/redis/cache/message"
	roomcache "IM_backend/internal/infrastructure/persistence/redis/cache/room"
	usercache "IM_backend/internal/infrastructure/persistence/redis/cache/user"
	"IM_backend/internal/infrastructure/ratelimit"
	realtimews "IM_backend/internal/infrastructure/realtime/websocket"
	authsvc "IM_backend/internal/infrastructure/security/auth"
	passwordsecurity "IM_backend/internal/infrastructure/security/password"
	minioobj "IM_backend/internal/infrastructure/storage/minio"
	"IM_backend/internal/shared/protocol"
	eventtransport "IM_backend/internal/transport/event"
	httpapi "IM_backend/internal/transport/http"
	userconversationhttp "IM_backend/internal/transport/http/conversation"
	filehttp "IM_backend/internal/transport/http/file"
	friendhttp "IM_backend/internal/transport/http/friend"
	friendrequesthttp "IM_backend/internal/transport/http/friend_request"
	messagehttp "IM_backend/internal/transport/http/message"
	"IM_backend/internal/transport/http/middleware"
	roomhttp "IM_backend/internal/transport/http/room"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	idGenerator, err := snow.NewGenerator(int(cfg.App.MachineID))
	if err != nil {
		log.Fatalln("初始化 ID 生成器失败：", err)
	}
	passwordHasher := passwordsecurity.NewHasher()

	realtimeGateway := realtimews.NewGateway()
	realtimeGateway.KeepAlive(ctx, cfg.WebSocket.TimerInterval, cfg.WebSocket.PongWaitSeconds)

	authCache := authcache.NewAuthCache(redisClient)
	roomCache := roomcache.NewRoomCache(redisClient)
	roomMemberCache := roomcache.NewRoomMemberCache(redisClient)
	fileCache := filecache.NewFileCache(redisClient)
	friendCache := friendcache.NewFriendCache(redisClient)
	messageCache := messagecache.NewMessageCache(redisClient)
	userCache := usercache.NewUserCache(redisClient)

	conversationCache := conversationcache.NewConversationCache(redisClient)

	kafkaClient, err := kafka.NewClient(cfg.Kafka)

	if err != nil {
		log.Fatalln("连接 Kafka 失败：", err)
	}
	defer func() {
		if err := kafkaClient.Close(); err != nil {
			log.Printf("关闭消息队列客户端失败：%v", err)
		}
	}()

	tokenIssuer := authsvc.NewTokenIssuer(authsvc.Options{
		Secret:          cfg.JWT.Secret,
		AccessTokenTTL:  time.Duration(cfg.JWT.AccessExpireMinutes) * time.Minute,
		RefreshTokenTTL: time.Duration(cfg.JWT.RefreshExpireHours) * time.Hour,
	})

	objectStorage, err := minioobj.NewObjectStorage(ctx, cfg.Storage.MinIO)
	if err != nil {
		log.Fatalln("连接 MinIO 失败：", err)
	}

	// 构造依赖
	fileRepository := filemysql.NewFileRepository(db)
	fileApplication := fileapp.NewApplication(fileapp.Options{
		MultipartTTL: time.Duration(cfg.Storage.MinIO.MultipartTTL) * time.Second,
		CacheTTL:     time.Duration(cfg.Storage.MinIO.CacheTTLSeconds) * time.Second,
		URLTTL:       time.Duration(cfg.Storage.MinIO.URLTTLSeconds) * time.Second,
	}, fileRepository, fileCache, objectStorage, idGenerator)
	fileHandle := filehttp.NewHandle(fileApplication)

	messageRepository := messagemysql.NewMessageRepository(db)
	messageImageRepository := messagemysql.NewMessageImageRepository(db)
	messageFileRepository := messagemysql.NewMessageFileRepository(db)
	messageStickerRepository := messagemysql.NewMessageStickerRepository(db)
	messageVideoRepository := messagemysql.NewMessageVideoRepository(db)
	outboxRepository := outboxmysql.NewRepository(db, idGenerator)
	conversationRepository := conversationmysql.NewConversationRepository(db)
	userConversationRepository := conversationmysql.NewUserConversationRepository(db)

	userRepository := usermysql.NewUserRepository(db)
	userApp := userapp.NewUserApplication(userRepository, userapp.Options{
		AccessTokenTTL:  time.Duration(cfg.JWT.AccessExpireMinutes) * time.Minute,
		RefreshTokenTTL: time.Duration(cfg.JWT.RefreshExpireHours) * time.Hour,
	}, authCache, tokenIssuer, idGenerator, passwordHasher)
	userHandle := userhttp.NewUserHandle(userApp)

	friendRepository := friendmysql.NewFriendRepository(db)
	friendRequestRepository := friendmysql.NewFriendRequestRepository(db)
	friendRequestApp := friendapp.NewRequestApplication(
		friendRequestRepository,
		userRepository,
		messageRepository,
		conversationRepository,
		userConversationRepository,
		conversationCache,
		friendRepository,
		friendCache,
		txManager,
		idGenerator,
		outboxRepository,
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
		conversationCache,
		roomCache,
		roomMemberCache,
		txManager,
		idGenerator,
		realtimeGateway,
		outboxRepository,
	)
	roomHandle := roomhttp.NewRoomHandle(roomApp)

	messageDelivery := messageapp.NewDelivery(
		realtimeGateway,
		roomRepository,
		roomUserRepository,
		roomMemberCache,
		messageapp.DeliveryOptions{
			RoomRealtimeFanoutLimit:           cfg.Message.RoomRealtimeFanoutLimit,
			LargeRoomNoticeLingerMilliseconds: cfg.Message.LargeRoomNoticeLingerMilliseconds,
			LargeRoomNoticeShardCount:         cfg.Message.LargeRoomNoticeShardCount,
			LargeRoomNoticeMaxPending:         cfg.Message.LargeRoomNoticeMaxPending,
		},
	)
	defer messageDelivery.Close(context.Background())
	messageSendHandler := eventtransport.NewMessageHandler(messageDelivery)
	readNotifyHandler := eventtransport.NewReadHandler(realtimeGateway)
	friendRequestHandler := eventtransport.NewFriendRequestHandler(realtimeGateway)
	roomMemberChangedHandler := eventtransport.NewRoomMemberChangedHandler(roomApp)
	messageProducer := kafka.NewProducer(kafkaClient, "msg")
	consumerRouter := kafka.NewConsumerRouter(map[string]eventbus.Handler{
		protocol.EventTypeSendMessage:      messageSendHandler,
		protocol.EventReadMessageCommitted: readNotifyHandler, // 当读水位提交后，将已读用户通知给消息发送方
		protocol.EventFriendRequestCreated: friendRequestHandler,
		protocol.EventRoomMemberChanged:    roomMemberChangedHandler,
	})

	messageConsumerGroup, err := kafka.NewConsumerGroup(kafkaClient, []string{
		string(protocol.EventReadMessageCommitted),
		string(protocol.EventTypeSendMessage),
		string(protocol.EventFriendRequestCreated),
		string(protocol.EventRoomMemberChanged),
	},
		consumerRouter,
		kafka.WithDeadLetterPublisher(messageProducer),
	)

	if err != nil {
		log.Fatal("创建消息消费组失败：", err)
	}

	go func() {
		if err := messageConsumerGroup.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("消息队列消费者已停止：%v", err)
		}
	}()

	dispatcher := ws.NewDispatcher()
	outboxWorker := outboxinfra.NewWorker(txManager, outboxRepository, messageProducer)

	go outboxWorker.Start(ctx)

	messageApplication := messageapp.NewMessageApplication(
		conversationCache,
		friendCache,
		messageCache,
		roomMemberCache,
		userCache,
		txManager,
		userConversationRepository,
		conversationRepository,
		outboxRepository,
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
		idGenerator,
		cfg,
	)
	messageHandle := messagehttp.NewMessageHandle(messageApplication)

	wsHandle := ws.NewWSHandler(
		messageApplication,
		cfg,
		dispatcher,
		realtimeGateway,
		idGenerator,
	)

	userConversationApplication := conversationapp.NewUserConvApplication(
		userConversationRepository,
		conversationRepository,
		messageRepository,
		friendRepository,
		userRepository,
		roomRepository,
	)

	userConversationHandler := userconversationhttp.NewUserConversationHandle(userConversationApplication)
	// testdataApplication := testdataapp.NewBootstrapApplication(
	// 	cfg,
	// 	db,
	// 	redisClient,
	// 	userRepository,
	// 	friendRepository,
	// 	messageRepository,
	// 	conversationRepository,
	// 	userConversationRepository,
	// 	roomRepository,
	// 	roomUserRepository,
	// 	roomApp,
	// 	messageApplication,
	// )
	// testdataHandle := testdatahttp.NewHandle(testdataApplication)

	// 注册中间件
	authMiddle := middleware.NewAuthMiddleware(cfg, authCache)

	limiter := ratelimit.NewRedisLimit(redisClient, "rate:limit")
	limiterMiddleware := middleware.NewLimitMiddleware(limiter, true)
	wsHandle.SetLimiter(limiter)

	// 注册路由
	apiGroup := r.Group("/api/v1")
	httpapi.RegisterFriendRequestRouter(apiGroup, friendRequestHandle, authMiddle)
	httpapi.RegisterUserRouter(apiGroup, userHandle, authMiddle, limiterMiddleware)
	httpapi.RegisterUserConversationRouter(apiGroup, userConversationHandler, authMiddle, limiterMiddleware)
	httpapi.RegisterFriendRouter(apiGroup, friendHandle, authMiddle)
	httpapi.RegisterRoomRouter(apiGroup, roomHandle, authMiddle)
	httpapi.RegisterMessagesRouter(apiGroup, messageHandle, authMiddle)
	httpapi.RegisterFileRouter(apiGroup, fileHandle, authMiddle)
	// if cfg.App.Env == "development" {
	// 	httpapi.RegisterTestDataRouter(apiGroup.Group("/dev"), testdataHandle)
	// }

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

	log.Println("服务已启动")

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("启动服务失败：", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Println("服务正在停止")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("关闭 HTTP 服务失败：%v", err)
	}
	if err := realtimeGateway.Shutdown(shutdownCtx); err != nil {
		log.Printf("关闭 WebSocket 会话失败：%v", err)
	}
	log.Println("服务已停止")
}
