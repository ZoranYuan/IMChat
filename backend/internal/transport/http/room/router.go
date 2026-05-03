package room

import "github.com/gin-gonic/gin"

func RegisterRoutes(rr gin.IRoutes, rh *RoomHandle) {
	rr.POST("", rh.Create)
	rr.GET("/:roomId/invite-code", rh.Invite)
	rr.POST("/join", rh.Join)
}
