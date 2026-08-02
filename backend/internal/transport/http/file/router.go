package file

import "github.com/gin-gonic/gin"

func RegisterRoutes(r gin.IRoutes, h *Handle) {
	r.POST("", h.Upload)
	r.POST("/multipart/init", h.InitMultipartUpload)
	r.POST("/multipart/:uploadId/parts/presign", h.PresignMultipartParts)
	r.POST("/multipart/:uploadId/complete", h.CompleteMultipartUpload)
	r.GET("/attachments/:attachmentId/access-url", h.GetAttachmentAccessURL)
}
