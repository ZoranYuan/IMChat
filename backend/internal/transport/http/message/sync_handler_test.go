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

func TestMessageRoutesExcludeWatchTogether(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/messages"), NewMessageHandle(nil))
	for _, path := range []string{"/messages/danmaku", "/messages/videos"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("removed route %s: expected 404, got %d", path, recorder.Code)
		}
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

func TestOfflineMessagesRequiresSnapshotSeq(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/message/offline?conversationId=room1", nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	ctx.Set("userId", "u1")

	NewMessageHandle(nil).GetOfflineMessages(ctx)

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

func TestOfflineMessageResponseCarriesCursor(t *testing.T) {
	result := toOfflineMessageResponse(
		[]messageapp.MessageDTO{{MessageID: "m1", Seq: 11}},
		15,
		true,
	)
	if len(result.Messages) != 1 || result.Messages[0].MessageID != "m1" {
		t.Fatalf("message mapping failed: %+v", result)
	}
	if result.NextCursor != 15 || !result.HasMore {
		t.Fatalf("offline cursor mapping failed: %+v", result)
	}
}
