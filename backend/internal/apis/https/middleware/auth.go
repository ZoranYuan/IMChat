package middleware

import (
	"IM_backend/configs"
	"IM_backend/internal/apis/response"
	auth_cache_interface "IM_backend/internal/applications/interface/cache/auth"
	"IM_backend/internal/infrastructure/pkg/jwt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthMiddleware struct {
	authCache auth_cache_interface.AuthCacheInterface
	config    configs.Config
}

func NewAuthMiddleware(cfg configs.Config, authCache auth_cache_interface.AuthCacheInterface) *AuthMiddleware {
	return &AuthMiddleware{
		config:    cfg,
		authCache: authCache,
	}
}

func (a *AuthMiddleware) JWTAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader("Authorization")[7:]
		if token == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
			return
		}

		claim, err := jwt.ValidateToken(token, a.config.JWT.Secret)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
			return
		}

		userId, err := a.authCache.GetUserIdByAccessToken(ctx, token)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, response.Error(http.StatusInternalServerError, "服务异常"))
			return
		}

		if userId != claim.UserID {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "未知 token"))
			return
		}

		ctx.Set("userId", userId)
		ctx.Next()
	}
}
