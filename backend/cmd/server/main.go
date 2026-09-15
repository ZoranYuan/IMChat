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
	filecleanup "IM_backend/internal/infrastructure/filecleanup"
	"IM_backend/internal/infrastructure/id/snow"
	"IM_backend/internal/infrastructure/mq/kafka"
	outboxinfra "IM_backend/internal/infrastructure/outbox"
	"IM_backend/internal/infrastructure/persistence"
	"IM_backend/internal/infrastructure/persistence/mysql"
	conversationmysql "IM_backend/internal/infrastructure/persistence/mysql/repository/conversation"
	filemysql "IM_backend/internal/infrastructure/persistence/mysql/repository/file"
	friendmysql "IM_backend/internal/infrastructure/persistence/mysql/repository/friend"
	inboxmysql "IM_backend/internal/infrastructure/persistence/mysql/repository/inbox"
	messagemysql "IM_backend/internal/infrastructure/persistence/mysql/repository/message"
	outboxmysql "IM_backend/internal/infrastructure/persistence/mysql/repository/outbox"
	roommysql "IM_backend/internal/infrastructure/persistence/mysql/repository/room"
	roomusermysql "IM_backend/internal/infrastructure/persistence/mysql/repository/room_user"
	usermysql "IM_backend/internal/infrastructure/persistence/mysql/repository/user"
	"IM_backend/internal/infrastructure/persistence/redis"
	authcache "IM_backend/internal/infrastructure/persistence/redis/cache/auth"
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
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	httpapi "IM_backend/internal/transport/http"
	userconversationhttp "IM_backend/internal/transport/http/conversation"
	filehttp "IM_backend/internal/transport/http/file"
	friendhttp "IM_backend/internal/transport/http/friend"
	friendrequesthttp "IM_backend/internal/transport/http/friend_request"
	messagehttp "IM_backend/internal/transport/http/message"
	"IM_backend/internal/transport/http/middleware"
	roomhttp "IM_backend/internal/transport/http/room"
	userhttp "IM_backend/internal/transport/http/user"
	mqhandler "IM_backend/internal/transport/mq/handler"
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
	if err := cfg.Validate(); err != nil {
		log.Fatal("配置校验失败：", err)
	}

	if cfg.App.Env == "development" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	redisClient := redis.InitRedis(cfg.Database.Redis.DSN)
	defer redisClient.Close()
	db := mysql.OpenMysql(cfg.Database.MySQL.DSN)
	dbSQL, err := db.DB()
	if err != nil {
		log.Fatal("获取数据库连接失败：", err)
	}
	defer dbSQL.Close()

	txManager := persistence.NewGormTxManager(db)
	idGenerator, err := snow.NewGenerator(int(cfg.App.MachineID))
	if err != nil {
		log.Fatalln("初始化 ID 生成器失败：", err)
	}
	passwordHasher := passwordsecurity.NewHasher()

	realtimeGateway := realtimews.NewGatewayWithRedis(ctx, redisClient)
	defer realtimeGateway.Close(context.Background())
	realtimeGateway.KeepAlive(ctx, cfg.WebSocket.TimerInterval, cfg.WebSocket.PongWaitSeconds)

	authCache := authcache.NewAuthCache(redisClient)
	roomCache := roomcache.NewRoomCache(redisClient, cfg.Message)
	roomMemberCache := roomcache.NewRoomMemberCache(redisClient, cfg.Message)
	fileCache := filecache.NewFileCache(redisClient)
	friendCache := friendcache.NewFriendCache(redisClient)
	messageCache := messagecache.NewMessageCache(redisClient)
	userCache := usercache.NewUserCache(redisClient)

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

	r.GET("/livez", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		checkCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		checks := gin.H{}
		if err := dbSQL.PingContext(checkCtx); err != nil {
			checks["mysql"] = err.Error()
		}
		if err := redisClient.Ping(checkCtx).Err(); err != nil {
			checks["redis"] = err.Error()
		}
		if err := objectStorage.Health(checkCtx); err != nil {
			checks["minio"] = err.Error()
		}
		if len(checks) > 0 {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": checks})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// 构造依赖
	fileRepository := filemysql.NewFileRepository(db)
	fileUploadRepository := filemysql.NewFileUploadRepository(db)
	messageAttachmentsRepository := messagemysql.NewMessageAttachmentRepository(db)
	fileApplication := fileapp.NewFileApplication(fileapp.Options{
		MultipartTTL:             time.Duration(cfg.Storage.MinIO.MultipartTTL) * time.Second,
		CacheTTL:                 time.Duration(cfg.Storage.MinIO.CacheTTLSeconds) * time.Second,
		AttachmentFileCardTTL:    time.Duration(cfg.Cache.AttachmentFileCard.TTLSeconds) * time.Second,
		URLTTL:                   time.Duration(cfg.Storage.MinIO.URLTTLSeconds) * time.Second,
		AttachmentAccessCacheTTL: time.Duration(cfg.Cache.AttachmentAccess.TTLSeconds) * time.Second,
		PartURLTTL:               time.Duration(cfg.Storage.MinIO.PartURLTTLSeconds) * time.Second,
		DirectUploadURLTTL:       time.Duration(cfg.Storage.MinIO.DirectUploadURLTTLSeconds) * time.Second,
		DirectUploadMaxSize:      cfg.Storage.MinIO.DirectUploadMaxSizeBytes,
		MultipartInitLockTTL:     time.Duration(cfg.Storage.MinIO.MultipartInitLockTTLSeconds) * time.Second,
		MultipartCompleteLockTTL: time.Duration(cfg.Storage.MinIO.MultipartCompleteLockTTLSeconds) * time.Second,
		MaxFileSize:              cfg.Storage.MinIO.MaxFileSizeBytes,
		MaxMultipartParts:        cfg.Storage.MinIO.MaxMultipartParts,
	}, fileRepository, fileUploadRepository, messageAttachmentsRepository, fileCache, objectStorage, idGenerator, txManager)

	// file 清除 worker
	multipartCleanupWorker := filecleanup.NewWorker(
		txManager,
		fileUploadRepository,
		fileRepository,
		fileCache,
		objectStorage,
		cfg.FileCleanup,
		time.Duration(cfg.Storage.MinIO.MultipartCompleteLockTTLSeconds)*time.Second,
	)
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		multipartCleanupWorker.Start(ctx)
	}()
	fileHandle := filehttp.NewHandle(fileApplication)

	messageRepository := messagemysql.NewMessageRepository(db)
	messageImageRepository := messagemysql.NewMessageImageRepository(db)
	messageFileRepository := messagemysql.NewMessageFileRepository(db)
	messageStickerRepository := messagemysql.NewMessageStickerRepository(db)
	messageVideoRepository := messagemysql.NewMessageVideoRepository(db)
	outboxRepository := outboxmysql.NewRepository(db, idGenerator)
	inboxRepository := inboxmysql.NewRepository(db)
	conversationRepository := conversationmysql.NewConversationRepository(db)
	userConversationRepository := conversationmysql.NewUserConversationRepository(db)

	userRepository := usermysql.NewUserRepository(db)
	userApp := userapp.NewUserApplication(userRepository, userapp.Options{
		AccessTokenTTL:  time.Duration(cfg.JWT.AccessExpireMinutes) * time.Minute,
		RefreshTokenTTL: time.Duration(cfg.JWT.RefreshExpireHours) * time.Hour,
	}, userCache, authCache, tokenIssuer, idGenerator, passwordHasher, txManager)
	userHandle := userhttp.NewUserHandle(
		userApp,
		time.Duration(cfg.JWT.AccessExpireMinutes)*time.Minute,
		time.Duration(cfg.JWT.RefreshExpireHours)*time.Hour,
	)

	friendRepository := friendmysql.NewFriendRepository(db)
	friendRequestRepository := friendmysql.NewFriendRequestRepository(db)
	friendRequestApp := friendapp.NewRequestApplication(
		friendRequestRepository,
		userRepository,
		messageRepository,
		conversationRepository,
		userConversationRepository,
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
		roomCache,
		roomMemberCache,
		txManager,
		idGenerator,
		realtimeGateway,
		outboxRepository,
	)
	roomHandle := roomhttp.NewRoomHandle(roomApp)

	messageDelivery := messageapp.NewMessageDelivery(
		realtimeGateway,
		roomRepository,
		roomCache,
		cfg.Message,
	)
	defer messageDelivery.Close(context.Background())
	messageSendHandler := mqhandler.NewMessageHandler(messageDelivery)
	readNotifyHandler := mqhandler.NewReadHandler(realtimeGateway)
	friendRequestHandler := mqhandler.NewFriendRequestHandler(realtimeGateway)
	roomMemberChangedHandler := mqhandler.NewRoomMemberChangedHandler(roomApp)
	fileCardWarmupHandler := mqhandler.NewFileCardWarmupHandler(
		fileCache,
		objectStorage,
		time.Duration(cfg.Cache.AttachmentFileCard.TTLSeconds)*time.Second,
		time.Duration(cfg.Storage.MinIO.URLTTLSeconds)*time.Second,
	)
	topicRouter, routerErr := kafka.NewTopicRouter(map[string]string{
		string(protocol.EventTypeSendMessage):      cfg.Kafka.Topics.Message,
		string(protocol.EventReadMessageCommitted): cfg.Kafka.Topics.ReadMessageCommitted,
		string(protocol.EventFriendRequestCreated): cfg.Kafka.Topics.FriendRequestCreated,
		string(protocol.EventRoomMemberChanged):    cfg.Kafka.Topics.RoomMemberChanged,
		string(protocol.EventFileCardWarmup):       cfg.Kafka.Topics.FileCardWarmup,
	})
	if routerErr != nil {
		log.Fatal("创建 Kafka Topic 路由失败：", routerErr)
	}
	messageProducer := kafka.NewProducer(kafkaClient, topicRouter)
	consumerRouter := kafka.NewConsumerRouter(map[string]eventbus.Handler{
		protocol.EventTypeSendMessage:      messageSendHandler,
		protocol.EventReadMessageCommitted: readNotifyHandler,
		protocol.EventFriendRequestCreated: friendRequestHandler,
		protocol.EventRoomMemberChanged:    roomMemberChangedHandler,
		protocol.EventFileCardWarmup:       fileCardWarmupHandler,
	},
		inboxRepository,
		txManager,
		cfg.Kafka.Consumer,
		messageProducer,
	)

	messageConsumerGroup, err := kafka.NewConsumerGroup(kafkaClient, topicRouter.Topics(),
		consumerRouter,
		topicRouter,
		cfg.Kafka.Consumer,
	)

	if err != nil {
		log.Fatal("创建消息消费组失败：", err)
	}

	// message 消费 worker
	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		if err := messageConsumerGroup.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("消息队列消费者已停止：%v", err)
		}
	}()

	dispatcher := ws.NewDispatcher()
	outboxWorker := outboxinfra.NewWorker(
		txManager,
		outboxRepository,
		messageProducer,
		cfg.Outbox,
	)

	// outbox worker
	outboxDone := make(chan struct{})
	go func() {
		defer close(outboxDone)
		outboxWorker.Start(ctx)
	}()

	messageApplication := messageapp.NewMessageApplication(
		friendCache,
		messageCache,
		roomMemberCache,
		roomCache,
		userCache,
		txManager,
		userConversationRepository,
		conversationRepository,
		outboxRepository,
		friendRepository,
		fileRepository,
		fileCache,
		objectStorage,
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
		messageAttachmentsRepository,
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
	limiterMiddleware := middleware.NewLimitMiddleware(limiter, false)
	wsHandle.SetLimiter(limiter)

	// 注册路由
	apiGroup := r.Group("/api/v1")
	apiGroup.Use(authMiddle.CookieOriginProtectionMiddleware())
	httpapi.RegisterFriendRequestRouter(apiGroup, friendRequestHandle, authMiddle)
	httpapi.RegisterUserRouter(apiGroup, userHandle, authMiddle, limiterMiddleware)
	httpapi.RegisterUserConversationRouter(apiGroup, userConversationHandler, authMiddle, limiterMiddleware)
	httpapi.RegisterFriendRouter(apiGroup, friendHandle, authMiddle)
	httpapi.RegisterRoomRouter(apiGroup, roomHandle, authMiddle)
	httpapi.RegisterMessagesRouter(apiGroup, messageHandle, authMiddle)
	httpapi.RegisterFileRouter(apiGroup, fileHandle, authMiddle, limiterMiddleware, shared_ratelimit.Policy{
		Rate:  cfg.Storage.MinIO.UploadRate,
		Burst: cfg.Storage.MinIO.UploadBurst,
	})
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("关闭 HTTP 服务失败：%v", err)
	}
	messageDelivery.Close(shutdownCtx)
	if err := realtimeGateway.Shutdown(shutdownCtx); err != nil {
		log.Printf("关闭 WebSocket 会话失败：%v", err)
	}
	cancel()
	for _, done := range []<-chan struct{}{cleanupDone, consumerDone, outboxDone} {
		select {
		case <-done:
		case <-shutdownCtx.Done():
			log.Printf("等待后台任务退出超时")
		}
	}
	log.Println("服务已停止")
}
