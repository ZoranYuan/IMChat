package mysql

import (
	"IM_backend/internal/infrastructure/persistence/mysql/model"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func OpenMySQL(dsn string) *gorm.DB {
	logLevel := logger.Info
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GORM_LOG_LEVEL"))) {
	case "silent":
		logLevel = logger.Silent
	case "error":
		logLevel = logger.Error
	case "warn", "warning":
		logLevel = logger.Warn
	case "", "info":
		logLevel = logger.Info
	}
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // 输出位置
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logLevel,
			Colorful:      true,
		},
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
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
	sqlDB.SetMaxIdleConns(50)                 // 空闲连接
	sqlDB.SetConnMaxLifetime(time.Minute * 4) // 连接复用时间

	return db
}

func InitMySQL(dsn string) *gorm.DB {
	db := OpenMySQL(dsn)
	if err := Migrate(db); err != nil {
		log.Fatal("数据库迁移失败：", err)
	}
	return db
}

// Migrate 按当前模型创建或更新表结构。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.OutboxRecord{},
		&model.InboxRecord{},
		&model.FriendRequest{},
		&model.Friend{},
		&model.Room{},
		&model.RoomUser{},
		&model.Conversation{},
		&model.UserConversation{},
		&model.Message{},
		&model.MessageSticker{},
		&model.MessageAttachment{},
		&model.File{},
		&model.FileUpload{},
		&model.SummaryRun{},
	)
}
