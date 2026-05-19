package file

import "github.com/gin-gonic/gin"

func RegisterRoutes(r gin.IRoutes, h *Handle) {
	r.POST("", h.Upload)
	r.GET("/:fileId", h.Get)
}
