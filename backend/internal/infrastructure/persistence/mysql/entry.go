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

func OpenMysql(dns string) *gorm.DB {
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

	return db
}

func InitMysql(dns string) *gorm.DB {
	db := OpenMysql(dns)
	if err := Migrate(db); err != nil {
		log.Fatal("数据库迁移失败：", err)
	}
	return db
}

func Migrate(db *gorm.DB) error {
	renameLegacyTables(db)
	prepareFriendIndexes(db)
	prepareFriendRequestUniquePair(db)
	prepareLegacyOutboxColumns(db)
	prepareFileUploadColumns(db)
	prepareFileHashColumn(db)
	if err := prepareMessageRequestHashColumn(db); err != nil {
		return err
	}
	if err := prepareMessageSequenceIndex(db); err != nil {
		return err
	}
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
		&model.MessageAttachment{},
		&model.File{},
		&model.FileUpload{},
	); err != nil {
		return err
	}
	if db.Migrator().HasTable("files") {
		if err := db.Exec("UPDATE files SET status = 'uploaded' WHERE status IS NULL OR status = ''").Error; err != nil {
			return err
		}
	}
	if err := backfillMemberAndOutboxState(db); err != nil {
		return err
	}
	if err := backfillFileUploadRetryState(db); err != nil {
		return err
	}
	if err := backfillMessageAttachments(db); err != nil {
		return err
	}
	dropLegacyColumns(db)
	return nil
}

func prepareMessageRequestHashColumn(db *gorm.DB) error {
	if !db.Migrator().HasTable("messages") || db.Migrator().HasColumn("messages", "request_hash") {
		return nil
	}
	return db.Exec(`
		ALTER TABLE messages
		ADD COLUMN request_hash VARCHAR(64) NOT NULL DEFAULT ''
	`).Error
}

// 历史版本可能把所有消息写成 seq=0。先按发送时间补齐会话序号，才能建立唯一索引。
func prepareMessageSequenceIndex(db *gorm.DB) error {
	if !db.Migrator().HasTable("messages") {
		return nil
	}

	var duplicateGroups int64
	if err := db.Raw(`
		SELECT COUNT(*)
		FROM (
			SELECT conversation_id, seq
			FROM messages
			GROUP BY conversation_id, seq
			HAVING COUNT(*) > 1
		) AS duplicated
	`).Scan(&duplicateGroups).Error; err != nil {
		return err
	}
	if duplicateGroups == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
			UPDATE messages AS m
			JOIN (
				SELECT ranked.message_id, ranked.new_seq
				FROM (
					SELECT message_id,
						ROW_NUMBER() OVER (
							PARTITION BY conversation_id
							ORDER BY send_time ASC, message_id ASC
						) AS new_seq
					FROM messages
				) AS ranked
			) AS repaired ON repaired.message_id = m.message_id
			SET m.seq = repaired.new_seq
		`).Error; err != nil {
			return err
		}

		return tx.Exec(`
			UPDATE conversations AS c
			JOIN (
				SELECT m.conversation_id, m.message_id, m.seq
				FROM messages AS m
				JOIN (
					SELECT conversation_id, MAX(seq) AS max_seq
					FROM messages
					GROUP BY conversation_id
				) AS latest
				  ON latest.conversation_id = m.conversation_id
				 AND latest.max_seq = m.seq
			) AS latest_message
			  ON latest_message.conversation_id = c.conversation_id
			SET c.latest_seq = latest_message.seq,
				c.latest_message_id = latest_message.message_id
		`).Error
	})
}

func backfillFileUploadRetryState(db *gorm.DB) error {
	if !db.Migrator().HasTable("file_uploads") {
		return nil
	}

	return db.Exec(`
		UPDATE file_uploads
		SET retry_count = 0,
		    next_retry_at = 0,
		    locked_at = NULL
		WHERE retry_count IS NULL
		   OR next_retry_at IS NULL
	`).Error
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

func prepareFileUploadColumns(db *gorm.DB) {
	if !db.Migrator().HasTable("file_uploads") {
		return
	}
	if db.Migrator().HasColumn("file_uploads", "size") && !db.Migrator().HasColumn("file_uploads", "expected_size") {
		if err := db.Migrator().RenameColumn("file_uploads", "size", "expected_size"); err != nil {
			log.Printf("重命名 file_uploads.size 为 expected_size 失败：%v", err)
		}
	}
}

func prepareFileHashColumn(db *gorm.DB) {
	if !db.Migrator().HasTable("files") || !db.Migrator().HasColumn("files", "file_hash") {
		return
	}
	if err := db.Exec(`
		UPDATE files
		SET file_hash = NULL
		WHERE file_hash = ''
	`).Error; err != nil {
		log.Printf("清理 files.file_hash 空值失败：%v", err)
	}
}

func backfillMemberAndOutboxState(db *gorm.DB) error {
	if err := db.Exec(`
		UPDATE room_members
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

func backfillMessageAttachments(db *gorm.DB) error {
	if !db.Migrator().HasTable("messages") || !db.Migrator().HasTable("message_attachments") {
		return nil
	}

	legacy := []struct {
		table string
		kind  int
		where string
	}{
		{table: "message_image_metadata", kind: 2, where: "mi.file_id IS NOT NULL AND mi.file_id <> ''"},
		{table: "message_video_metadata", kind: 3, where: "mv.file_id IS NOT NULL AND mv.file_id <> ''"},
		{table: "message_file_metadata", kind: 5, where: "mf.file_id IS NOT NULL AND mf.file_id <> ''"},
	}

	for _, item := range legacy {
		if !db.Migrator().HasTable(item.table) || !db.Migrator().HasColumn(item.table, "file_id") {
			continue
		}

		query := ""
		switch item.table {
		case "message_image_metadata":
			query = `
				INSERT INTO message_attachments
					(attachment_id, message_id, conversation_id, file_id, kind, expire_at, created_at)
				SELECT REPLACE(UUID(), '-', ''), m.message_id, m.conversation_id, mi.file_id, ?,
					CAST(UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000 AS UNSIGNED) + 1209600000,
					CAST(UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000 AS UNSIGNED)
				FROM message_image_metadata mi
				JOIN messages m ON m.message_id = mi.message_id
				WHERE ` + item.where + `
				  AND NOT EXISTS (
					  SELECT 1 FROM message_attachments ma
					  WHERE ma.message_id = mi.message_id AND ma.kind = ?
				  )`
		case "message_video_metadata":
			query = `
				INSERT INTO message_attachments
					(attachment_id, message_id, conversation_id, file_id, kind, expire_at, created_at)
				SELECT REPLACE(UUID(), '-', ''), m.message_id, m.conversation_id, mv.file_id, ?,
					CAST(UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000 AS UNSIGNED) + 1209600000,
					CAST(UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000 AS UNSIGNED)
				FROM message_video_metadata mv
				JOIN messages m ON m.message_id = mv.message_id
				WHERE ` + item.where + `
				  AND NOT EXISTS (
					  SELECT 1 FROM message_attachments ma
					  WHERE ma.message_id = mv.message_id AND ma.kind = ?
				  )`
		case "message_file_metadata":
			query = `
				INSERT INTO message_attachments
					(attachment_id, message_id, conversation_id, file_id, kind, expire_at, created_at)
				SELECT REPLACE(UUID(), '-', ''), m.message_id, m.conversation_id, mf.file_id, ?,
					CAST(UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000 AS UNSIGNED) + 1209600000,
					CAST(UNIX_TIMESTAMP(CURRENT_TIMESTAMP(3)) * 1000 AS UNSIGNED)
				FROM message_file_metadata mf
				JOIN messages m ON m.message_id = mf.message_id
				WHERE ` + item.where + `
				  AND NOT EXISTS (
					  SELECT 1 FROM message_attachments ma
					  WHERE ma.message_id = mf.message_id AND ma.kind = ?
				  )`
		}

		if err := db.Exec(query, item.kind, item.kind).Error; err != nil {
			return err
		}
	}
	return nil
}

func renameLegacyTables(db *gorm.DB) {
	renames := map[string]string{
		"message_outboxes": "outboxes",
		"user":             "users",
		"friend":           "friendships",
		"friend_request":   "friend_requests",
		"room":             "rooms",
		"room_user":        "room_members",
		"message_images":   "message_image_metadata",
		"message_videos":   "message_video_metadata",
		"message_files":    "message_file_metadata",
	}
	for oldName, newName := range renames {
		if !db.Migrator().HasTable(oldName) || db.Migrator().HasTable(newName) {
			continue
		}
		if err := db.Migrator().RenameTable(oldName, newName); err != nil {
			log.Printf("重命名 %s 为 %s 失败：%v", oldName, newName, err)
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
		log.Printf("查询 friendships 历史唯一索引失败：%v", err)
		return
	}

	for _, index := range indexes {
		if err := db.Migrator().DropIndex(&model.Friend{}, index.IndexName); err != nil {
			log.Printf("删除 friendships.%s 历史唯一索引失败：%v", index.IndexName, err)
		}
	}
}

func prepareFriendRequestUniquePair(db *gorm.DB) {
	if !db.Migrator().HasTable(&model.FriendRequest{}) {
		return
	}
	if db.Migrator().HasIndex(&model.FriendRequest{}, "idx_from_to") {
		if err := db.Migrator().DropIndex(&model.FriendRequest{}, "idx_from_to"); err != nil {
			log.Printf("删除 friend_requests.idx_from_to 失败：%v", err)
		}
	}
	if err := db.Exec(`
		DELETE fr
		FROM friend_requests fr
		JOIN friend_requests newer
		  ON newer.from_user_id = fr.from_user_id
		 AND newer.to_user_id = fr.to_user_id
		 AND (
		     newer.created_at > fr.created_at
		  OR (newer.created_at = fr.created_at AND newer.request_id > fr.request_id)
		 )
	`).Error; err != nil {
		log.Printf("清理重复 friend_requests 失败：%v", err)
	}
}

func dropLegacyColumns(db *gorm.DB) {
	for _, table := range []string{
		"files",
		"message_image_metadata",
		"message_video_metadata",
		"message_file_metadata",
		"message_stickers",
	} {
		if db.Migrator().HasColumn(table, "url") {
			if err := db.Migrator().DropColumn(table, "url"); err != nil {
				log.Printf("删除 %s.url 失败：%v", table, err)
			}
		}
	}
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

	legacyMessageColumns := map[string][]string{
		"message_image_metadata": {"file_id", "thumb_file_id"},
		"message_video_metadata": {"file_id", "cover_file_id"},
		"message_file_metadata":  {"file_id", "file_name", "mime_type", "size"},
	}
	for table, columns := range legacyMessageColumns {
		for _, column := range columns {
			if !db.Migrator().HasColumn(table, column) {
				continue
			}
			if err := db.Migrator().DropColumn(table, column); err != nil {
				log.Printf("删除 %s.%s 失败：%v", table, column, err)
			}
		}
	}

	// file_uploads.file_name 仍是当前 FileUpload 模型和上传完成流程的业务字段，
	// 不能按历史字段删除；bucket、size 才是已废弃的旧字段。
	for _, column := range []string{"bucket", "size"} {
		if !db.Migrator().HasColumn("file_uploads", column) {
			continue
		}
		if err := db.Migrator().DropColumn("file_uploads", column); err != nil {
			log.Printf("删除 file_uploads.%s 失败：%v", column, err)
		}
	}
}
