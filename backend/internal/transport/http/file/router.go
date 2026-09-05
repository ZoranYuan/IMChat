package file

import (
	shared_ratelimit "IM_backend/internal/shared/ratelimit"
	"IM_backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r gin.IRoutes, h *Handle, limiter *middleware.LimitMiddleware, policy shared_ratelimit.Policy) {
	if policy.Rate <= 0 {
		policy.Rate = 1
	}
	if policy.Burst <= 0 {
		policy.Burst = 5
	}
	r.POST("/direct/init", limiter.ByIP("file_upload_direct_init", policy), h.InitDirectUpload)
	r.POST("/direct/:uploadId/complete", limiter.ByIP("file_upload_direct_complete", policy), h.CompleteDirectUpload)
	r.POST("/multipart/init", limiter.ByIP("file_upload_init", policy), h.InitMultipartUpload)
	r.POST("/multipart/:uploadId/parts/presign", limiter.ByIP("file_upload_presign", policy), h.PresignMultipartParts)
	r.POST("/multipart/:uploadId/complete", limiter.ByIP("file_upload_complete", policy), h.CompleteMultipartUpload)
	r.POST("/attachments/access-urls", limiter.ByIP("file_attachment_access_urls", policy), h.GetAttachmentAccessURLs)
	r.GET("/attachments/:attachmentId/access-url", h.GetAttachmentAccessURL)
}
