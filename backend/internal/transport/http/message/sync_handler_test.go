package message

import (
	messageapp "IM_backend/internal/application/message"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSyncMessagesRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/message/sync?conversationId=room1", nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	NewMessageHandle(nil).SyncMessages(ctx)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestSyncMessagesRequiresConversationID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/message/sync?afterSeq=10", nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	ctx.Set("userId", "u1")

	NewMessageHandle(nil).SyncMessages(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestSyncMessageResponseCarriesCursor(t *testing.T) {
	result := toSyncMessageRes([]messageapp.MessageAppeDTO{{MessageId: "m1", Seq: 11}}, 11, true)
	if len(result.Messages) != 1 || result.Messages[0].MessageId != "m1" {
		t.Fatalf("message mapping failed: %+v", result)
	}
	if result.NextSeq != 11 || !result.HasMore {
		t.Fatalf("cursor mapping failed: %+v", result)
	}
}
