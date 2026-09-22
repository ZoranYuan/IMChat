package agent

import (
	"IM_backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, h *Handle, auth *middleware.AuthMiddleware) {
	group := r.Group("/rooms/:roomId/summaries").Use(auth.JWTAuthMiddleware())
	group.POST("", h.InitSummaryRun)
	group.GET("/:summaryRunId/events", h.AgentStream)
	group.POST("/:summaryRunId/retry", h.RetrySummary)
	group.POST("/:summaryRunId/cancel", h.CancelSummary)
}
