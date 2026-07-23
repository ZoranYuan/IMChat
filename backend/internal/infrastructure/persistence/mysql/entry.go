package mysql

import (
	"IM_backend/internal/infrastructure/persistence/mysql/model"
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
		log.Fatal("初始化 MySQL 失败：", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}

	sqlDB.SetMaxOpenConns(50)                 // 最大连接数（关键）
	sqlDB.SetMaxIdleConns(20)                 // 空闲连接
	sqlDB.SetConnMaxLifetime(time.Minute * 4) // 连接复用时间

	db.AutoMigrate(
		&model.User{},
		&model.OutboxRecord{},
		&model.FriendRequest{},
		&model.Friend{},
		&model.Room{},
		&model.RoomUser{},
		&model.Conversation{},
		&model.UserConversation{},
		&model.Message{},
		&model.MessageImage{},
		&model.MessageFile{},
		&model.MessageSticker{},
		&model.MessageVideo{},
		&model.File{},
	)
	dropLegacyColumns(db)
	return db
}

func dropLegacyColumns(db *gorm.DB) {
	if db.Migrator().HasColumn(&model.UserConversation{}, "latest_sync_seq") {
		if err := db.Migrator().DropColumn(&model.UserConversation{}, "latest_sync_seq"); err != nil {
			log.Printf("删除 user_conversations.latest_sync_seq 失败：%v", err)
		}
	}
}
