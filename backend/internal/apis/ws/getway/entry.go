package ws

import "github.com/gin-gonic/gin"

func RegisterWsRouter(r *gin.Engine, wh *WsHandle) {
	r.GET("/ws", wh.Handle)
}
