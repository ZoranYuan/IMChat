package file

import "github.com/gin-gonic/gin"

func RegisterRoutes(r gin.IRoutes, h *Handle) {
	r.POST("", h.Upload)
	r.POST("/multipart/init", h.InitMultipartUpload)
	r.PUT("/multipart/:uploadId/parts/:partNumber", h.UploadMultipartPart)
	r.POST("/multipart/:uploadId/complete", h.CompleteMultipartUpload)
	r.GET("/:fileId", h.Get)
}
