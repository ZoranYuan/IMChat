package middleware

import (
	"IM_backend/configs"
	auth_cache_interface "IM_backend/internal/application/ports/cache/auth"
	"IM_backend/internal/infrastructure/security/jwt"
	"IM_backend/internal/interfaces/http/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuthMiddleware struct {
	authCache auth_cache_interface.AuthCache
	config    configs.Config
}

func NewAuthMiddleware(cfg configs.Config, authCache auth_cache_interface.AuthCache) *AuthMiddleware {
	return &AuthMiddleware{
		config:    cfg,
		authCache: authCache,
	}
}

func (a *AuthMiddleware) JWTAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader("Authorization")

		if token == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
			return
		}

		if !strings.HasPrefix(token, "Bearer ") {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
			return
		}

		token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
		if token == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
			return
		}

		claim, err := jwt.ValidateToken(token, a.config.JWT.Secret)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
			return
		}

		userId, err := a.authCache.GetUserIDByAccessToken(ctx, token)
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
