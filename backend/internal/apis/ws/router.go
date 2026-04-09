package ws

import (
	"IM_backend/internal/apis/https/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterWsRouter(r *gin.RouterGroup, wh *WsHandler, auth *middleware.AuthMiddleware) {
	r.Use(auth.JWTAuthMiddleware()).GET("/ws", wh.Handler)
}
