package ws

import (
	"github.com/gin-gonic/gin"
)

func RegisterWsRouter(r gin.IRoutes, wh *WsHandler) {
	r.GET("/ws", wh.Handler)
}
