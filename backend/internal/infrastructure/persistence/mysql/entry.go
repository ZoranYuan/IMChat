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

	renameLegacyTables(db)
	prepareFriendIndexes(db)
	prepareFriendRequestUniquePair(db)
	prepareLegacyOutboxColumns(db)

	if err := db.AutoMigrate(
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
		&model.MessageImage{},
		&model.MessageFile{},
		&model.MessageSticker{},
		&model.MessageVideo{},
		&model.File{},
	); err != nil {
		log.Fatalf("数据库结构迁移失败：%v", err)
	}
	if err := backfillMemberAndOutboxState(db); err != nil {
		log.Fatalf("数据库历史数据迁移失败：%v", err)
	}
	dropLegacyColumns(db)
	return db
}

func prepareLegacyOutboxColumns(db *gorm.DB) {
	if !db.Migrator().HasTable(&model.OutboxRecord{}) {
		return
	}
	if db.Migrator().HasColumn(&model.OutboxRecord{}, "status") {
		if err := db.Exec(`
			UPDATE outboxes
			SET status = 'pending'
			WHERE status IS NULL OR status = ''
		`).Error; err != nil {
			log.Fatalf("清理历史 Outbox 状态失败：%v", err)
		}
	}
	if db.Migrator().HasColumn(&model.OutboxRecord{}, "next_retry_at") {
		if err := db.Exec(`
			ALTER TABLE outboxes
			MODIFY COLUMN next_retry_at DATETIME(3) NULL DEFAULT NULL
		`).Error; err != nil {
			log.Fatalf("调整历史 Outbox 重试时间字段失败：%v", err)
		}
		if err := db.Exec(`
			UPDATE outboxes
			SET next_retry_at = NOW()
			WHERE next_retry_at IS NULL OR next_retry_at < '1970-01-01 00:00:00'
		`).Error; err != nil {
			log.Fatalf("清理历史 Outbox 重试时间失败：%v", err)
		}
	}
}

func backfillMemberAndOutboxState(db *gorm.DB) error {
	if err := db.Exec(`
		UPDATE room_user
		SET version = 1
		WHERE version IS NULL OR version <= 0
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		UPDATE outboxes
		SET status = 'pending'
		WHERE status IS NULL OR status = ''
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		UPDATE outboxes
		SET next_retry_at = NOW()
		WHERE next_retry_at IS NULL OR next_retry_at < '1970-01-01 00:00:00'
	`).Error; err != nil {
		return err
	}
	return db.Exec(`
		UPDATE outboxes
		SET topic = event_type
		WHERE topic IS NULL OR topic = ''
	`).Error
}

func renameLegacyTables(db *gorm.DB) {
	if db.Migrator().HasTable("message_outboxes") && !db.Migrator().HasTable("outboxes") {
		if err := db.Migrator().RenameTable("message_outboxes", "outboxes"); err != nil {
			log.Printf("重命名 message_outboxes 为 outboxes 失败：%v", err)
		}
	}
}

func prepareFriendIndexes(db *gorm.DB) {
	if !db.Migrator().HasTable(&model.Friend{}) {
		return
	}

	var indexes []struct {
		IndexName string
	}
	if err := db.Raw(`
		SELECT DISTINCT INDEX_NAME AS index_name
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = ?
		  AND NON_UNIQUE = 0
		  AND INDEX_NAME <> 'PRIMARY'
	`, (&model.Friend{}).TableName()).Scan(&indexes).Error; err != nil {
		log.Printf("查询 friend 历史唯一索引失败：%v", err)
		return
	}

	for _, index := range indexes {
		if err := db.Migrator().DropIndex(&model.Friend{}, index.IndexName); err != nil {
			log.Printf("删除 friend.%s 历史唯一索引失败：%v", index.IndexName, err)
		}
	}
}

func prepareFriendRequestUniquePair(db *gorm.DB) {
	if !db.Migrator().HasTable(&model.FriendRequest{}) {
		return
	}
	if db.Migrator().HasIndex(&model.FriendRequest{}, "idx_from_to") {
		if err := db.Migrator().DropIndex(&model.FriendRequest{}, "idx_from_to"); err != nil {
			log.Printf("删除 friend_request.idx_from_to 失败：%v", err)
		}
	}
	if err := db.Exec(`
		DELETE fr
		FROM friend_request fr
		JOIN friend_request newer
		  ON newer.from_user_id = fr.from_user_id
		 AND newer.to_user_id = fr.to_user_id
		 AND (
		     newer.created_at > fr.created_at
		  OR (newer.created_at = fr.created_at AND newer.request_id > fr.request_id)
		 )
	`).Error; err != nil {
		log.Printf("清理重复 friend_request 失败：%v", err)
	}
}

func dropLegacyColumns(db *gorm.DB) {
	if db.Migrator().HasColumn(&model.FriendRequest{}, "active_key") {
		if err := db.Migrator().DropColumn(&model.FriendRequest{}, "active_key"); err != nil {
			log.Printf("删除 friend_request.active_key 失败：%v", err)
		}
	}
	if db.Migrator().HasColumn(&model.UserConversation{}, "latest_sync_seq") {
		if err := db.Migrator().DropColumn(&model.UserConversation{}, "latest_sync_seq"); err != nil {
			log.Printf("删除 user_conversations.latest_sync_seq 失败：%v", err)
		}
	}
}
