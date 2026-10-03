package file

import (
	sharedratelimit "IM_backend/internal/shared/ratelimit"
	"IM_backend/internal/transport/http/middleware"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type allowedUploadLimiter struct{}

func (allowedUploadLimiter) Allow(context.Context, string, sharedratelimit.Policy) (sharedratelimit.Decision, error) {
	return sharedratelimit.Decision{Allowed: true}, nil
}

func TestUploadRoutesUseUnifiedEntry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	limiter := middleware.NewLimitMiddleware(allowedUploadLimiter{}, false)
	RegisterRoutes(router.Group("/files"), NewHandle(nil), limiter, sharedratelimit.Policy{Rate: 1, Burst: 5})
	for _, test := range []struct {
		path string
		want int
	}{
		{"/files/uploads/init", http.StatusUnauthorized},
		{"/files/uploads/upload-1/complete", http.StatusUnauthorized},
		{"/files/uploads/upload-1/parts/presign", http.StatusUnauthorized},
		{"/files/direct/init", http.StatusNotFound},
		{"/files/direct/upload-1/complete", http.StatusNotFound},
		{"/files/multipart/init", http.StatusNotFound},
		{"/files/multipart/upload-1/complete", http.StatusNotFound},
		{"/files/multipart/upload-1/parts/presign", http.StatusNotFound},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, test.path, nil))
		if recorder.Code != test.want {
			t.Fatalf("%s：期望 %d，实际 %d", test.path, test.want, recorder.Code)
		}
	}
}
