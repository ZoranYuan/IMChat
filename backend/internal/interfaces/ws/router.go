package ws

import (
	"IM_backend/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterWSRouter(r *gin.RouterGroup, wh *WSHandler, auth *middleware.AuthMiddleware) {
	r.Use(auth.JWTAuthMiddleware()).GET("/ws", wh.Handler)
}
