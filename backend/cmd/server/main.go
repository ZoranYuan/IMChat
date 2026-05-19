package main

import (
	"IM_backend/configs"
	fileapp "IM_backend/internal/application/file"
	friendapp "IM_backend/internal/application/friend"
	friendrequestapp "IM_backend/internal/application/friend_request"
	messageapp "IM_backend/internal/application/message"
	roomapp "IM_backend/internal/application/room"
	userapp "IM_backend/internal/application/user"
	"IM_backend/internal/infrastructure/messaging"
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
	"IM_backend/internal/infrastructure/persistence/redis/cache/local"
	roomredis "IM_backend/internal/infrastructure/persistence/redis/cache/room"
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
	userhttp "IM_backend/internal/transport/http/user"
	"IM_backend/internal/transport/ws"
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

	localConvVersionCache := local.NewConversationVersionTTLCache(60*time.Second, 1000, 30*time.Second)
	localConvVersionCache.StartCleanup(ctx)

	gateway := ws.NewGateway()
	gateway.KeepAlive(cfg.WebSocket.TimerInterval, cfg.WebSocket.PongWaitSeconds)

	authCache := authredis.NewAuthCache(redis)
	roomCache := roomredis.NewRoomCache(redis)
	fileCache := fileredis.NewFileCache(redis)

	conversationCache := conversationredis.NewConversationCache(redis)

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
	conversationRepository := messagemysql.NewConversationRepository(db)
	userConversationRepository := messagemysql.NewUserConversationRepository(db)

	userRepository := usermysql.NewUserRepository(db)
	userApp := userapp.NewUserApplication(userRepository, cfg, authCache, authService)
	userHandle := userhttp.NewUserHandle(userApp)

	friendRepository := friendmysql.NewFriendRepository(db)
	friendRequestRepositoy := friendrequestmysql.NewFriendRequestRepository(db)
	friendRequestApp := friendrequestapp.NewFriendApplication(
		friendRequestRepositoy,
		userRepository,
		cfg,
		friendRepository,
		conversationCache,
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
		conversationCache,
		localConvVersionCache,
		txManager,
	)
	roomHandle := roomhttp.NewRoomHandle(roomApp)

	groupHandler := kafka.NewGroupHandler(
		gateway,
		roomRepository,
		userConversationRepository,
		conversationCache,
		localConvVersionCache,
	)

	messageConsumer := kafka.NewConsumer(kafkaClient, []string{
		string(protocol.EventMessageReadAck),
		string(protocol.EventTypeMessage),
		string(protocol.EventTypeMsgAck),
		string(protocol.EventConversationSyncSeq),
	},
		fmt.Sprintf("machine-%d-group", cfg.App.MachineID),
		groupHandler,
	)

	go messageConsumer.Start(ctx)

	messageProducer := kafka.NewProducer(kafkaClient, "msg")

	dispatcher := ws.NewDispatcher()
	taskManager := mq.NewTaskManager(messageProducer)

	messageApplication := messageapp.NewMessageApplication(
		cfg,
		conversationCache,
		txManager,
		taskManager,
		userConversationRepository,
		conversationRepository,
		friendRepository,
		messageRepository,
		roomUserRepository,
		roomRepository,
		localConvVersionCache,
	)
	messageHandle := messagehttp.NewMessageHandle(messageApplication)

	wsHandle := ws.NewWSHandler(
		messageApplication,
		cfg,
		dispatcher,
		gateway,
	)

	// 注册中间件
	authMiddle := middleware.NewAuthMiddleware(cfg, authCache)

	// 注册路由
	apiGroup := r.Group("/api/v1")
	httpapi.RegisterFriendRequestRouter(apiGroup, friendRequestHandle, authMiddle)
	httpapi.RegisterUserRouter(apiGroup, userHandle, authMiddle)
	httpapi.RegisterFriendRouter(apiGroup, friendHandle, authMiddle)
	httpapi.RegisterRoomRouter(apiGroup, roomHandle, authMiddle)
	httpapi.RegisterMessagesRouter(apiGroup, messageHandle, authMiddle)
	httpapi.RegisterFileRouter(apiGroup, fileHandle, authMiddle)

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
