package middleware

import (
	"IM_backend/configs"
	"IM_backend/internal/infrastructure/security/jwt"
	"IM_backend/internal/transport/http/response"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type AuthMiddleware struct {
	config configs.Config
}

func NewAuthMiddleware(cfg configs.Config) *AuthMiddleware {
	return &AuthMiddleware{
		config: cfg,
	}
}

func (a *AuthMiddleware) JWTAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader("Authorization")

		if token != "" {
			if !strings.HasPrefix(token, "Bearer ") {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
				return
			}
			token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
		} else if cookieToken, err := ctx.Cookie("access_token"); err == nil {
			token = cookieToken
		} else if strings.EqualFold(ctx.GetHeader("Upgrade"), "websocket") {
			// 兼容旧客户端；浏览器客户端优先使用 HttpOnly Cookie。
			token = ctx.Query("token")
		}

		token = strings.TrimSpace(token)
		if token == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
			return
		}

		claim, err := jwt.ValidateToken(token, a.config.JWT.Secret)
		if err != nil || claim.TokenType != "access" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, response.Error(http.StatusUnauthorized, "登录过期"))
			return
		}

		ctx.Set("userId", claim.UserID)
		ctx.Set("sessionId", claim.SessionID)
		ctx.Next()
	}
}
