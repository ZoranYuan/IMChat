package main

import (
	"IM_backend/configs"
	apis "IM_backend/internal/apis/https"
	https_user "IM_backend/internal/apis/https/user"
	"IM_backend/internal/apis/ws"
	"IM_backend/internal/applications"
	"IM_backend/internal/infrastructure/database/mysql"
	"IM_backend/internal/infrastructure/database/mysql/repository"
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

	cfg := configs.LoadConfig("/workspace/IM/backend/configs/config.yaml")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	if cfg.App.Env == "development" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	db := mysql.InitMysql(cfg.Database.MySQL.DSN)
	defer func() {
		db = nil
		cancel()
	}()

	// 构造依赖
	userRepository := repository.NewUserRepository(db)
	userApp := applications.NewUserApplication(userRepository)
	userHandler := https_user.NewUserHandler(userApp)

	// 注册路由
	apis.RegisterUserRouter(r, userHandler)
	ws.RegisterWsRouter(r)

	srv := &http.Server{
		Addr:    cfg.Server.Port,
		Handler: r,
	}

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
