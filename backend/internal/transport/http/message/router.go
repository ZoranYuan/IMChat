package message

import "github.com/gin-gonic/gin"

func RegisterRoutes(mr gin.IRoutes, mh *MessageHandle) {
	mr.GET("/history", mh.GetHistoryMessages)
	mr.GET("/offline", mh.GetOfflineMessages)
}
