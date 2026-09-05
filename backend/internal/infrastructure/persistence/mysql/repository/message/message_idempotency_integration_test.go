package message_test

import (
	messageentity "IM_backend/internal/domain/message/entity"
	messagevo "IM_backend/internal/domain/message/value_object"
	mysqlrepo "IM_backend/internal/infrastructure/persistence/mysql/repository/message"
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openIdempotencyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("IM_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 IM_TEST_MYSQL_DSN，跳过 MySQL 幂等集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("连接 MySQL 测试库失败: %v", err)
	}
	return db
}

func testMessage(id, clientID string) *messageentity.Message {
	return &messageentity.Message{
		MessageId:      id,
		ConversationId: "idempotency-test-conversation",
		SenderId:       "idempotency-test-sender",
		ClientMsgId:    &clientID,
		Seq:            1,
		Type:           messagevo.Text,
		Content:        "idempotency-test",
		Status:         messagevo.Normal,
		SendTime:       1,
	}
}

func TestMessageRepositoryDuplicateClientMessage(t *testing.T) {
	db := openIdempotencyTestDB(t)
	cleanup := db.Exec("DELETE FROM messages WHERE message_id LIKE 'idem-test-%'")
	if cleanup.Error != nil {
		t.Fatalf("清理测试数据失败: %v", cleanup.Error)
	}
	t.Cleanup(func() { db.Exec("DELETE FROM messages WHERE message_id LIKE 'idem-test-%'") })

	repo := mysqlrepo.NewMessageRepository(db)
	ctx := context.Background()
	if err := repo.CreateNewMessage(ctx, testMessage("idem-test-first", "client-test-1")); err != nil {
		t.Fatalf("写入第一条消息失败: %v", err)
	}

	err := repo.CreateNewMessage(ctx, testMessage("idem-test-second", "client-test-1"))
	if err != messageentity.ErrDuplicateClientMessage {
		t.Fatalf("重复 clientMsgId 应返回幂等错误，实际=%v", err)
	}

	existing, err := repo.FindByClientMsgID(ctx, "idempotency-test-sender", "client-test-1")
	if err != nil || existing == nil || existing.MessageId != "idem-test-first" {
		t.Fatalf("重复请求未回查到首条消息: message=%+v err=%v", existing, err)
	}
}

func TestMessageRepositoryConcurrentClientMessage(t *testing.T) {
	db := openIdempotencyTestDB(t)
	db.Exec("DELETE FROM messages WHERE message_id LIKE 'idem-test-concurrent-%'")
	t.Cleanup(func() { db.Exec("DELETE FROM messages WHERE message_id LIKE 'idem-test-concurrent-%'") })

	repo := mysqlrepo.NewMessageRepository(db)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := repo.CreateNewMessage(ctx, testMessage(
				fmt.Sprintf("idem-test-concurrent-%d", i),
				"client-test-concurrent",
			))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	var success, duplicate int
	for err := range errs {
		switch err {
		case nil:
			success++
		case messageentity.ErrDuplicateClientMessage:
			duplicate++
		default:
			t.Fatalf("并发写入返回未知错误: %v", err)
		}
	}
	if success != 1 || duplicate != 1 {
		t.Fatalf("并发幂等结果错误: success=%d duplicate=%d", success, duplicate)
	}
}
