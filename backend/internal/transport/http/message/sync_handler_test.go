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

func TestSyncMessageResponseCarriesMessages(t *testing.T) {
	result := toSyncMessageResponse([]messageapp.MessageDTO{{MessageID: "m1", Seq: 11}})
	if len(result.Messages) != 1 || result.Messages[0].MessageID != "m1" {
		t.Fatalf("message mapping failed: %+v", result)
	}
}
