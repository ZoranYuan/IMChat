package testdata

import "github.com/gin-gonic/gin"

func RegisterRoutes(r gin.IRoutes, h *Handle) {
	r.POST("/rebuild", h.Rebuild)
}
