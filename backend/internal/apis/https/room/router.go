package https_room

import "github.com/gin-gonic/gin"

func AddRoomRouter(rr gin.IRoutes, rh *RoomHandle) {
	rr.POST("/create", rh.Create)
	rr.GET("/invite/:roomId", rh.Invite)
	rr.POST("/join/:roomId", rh.Join)
}
