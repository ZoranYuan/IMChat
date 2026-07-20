package conversation

import "github.com/gin-gonic/gin"

func RegisterRoutes(r gin.IRoutes, h *UserConversationHandle) {
	r.GET("", h.GetUserConversations)
}
