package mysql

import (
	"IM_backend/internal/infrastructure/database/mysql/model"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitMysql(dns string) *gorm.DB {
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // 输出位置
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logger.Info,
			Colorful:      true,
		},
	)

	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{
		Logger: gormLogger,
	})

	if err != nil {
		log.Fatal("filed to init mysql", err)
	}

	db.AutoMigrate(&model.User{}, &model.FriendRequest{}, &model.Friend{})
	return db
}
