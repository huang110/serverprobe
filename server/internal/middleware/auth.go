package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const contextAuthenticatedKey = "authenticated"

// JWTAuth HTTP JWT 鉴权中间件。
// 从 Authorization: Bearer <token> 或 Authorization: <token> 中读取 Token。
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "unauthorized",
			})
			return
		}

		claims, err := ValidateJWT(token, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "unauthorized",
			})
			return
		}

		setClaims(c, claims)
		c.Next()
	}
}

// OptionalJWTAuth 可选 JWT 中间件。
// 无 Token 或 Token 无效时按游客继续访问；有效 Token 会写入登录态。
func OptionalJWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token != "" {
			if claims, err := ValidateJWT(token, secret); err == nil {
				setClaims(c, claims)
			}
		}
		c.Next()
	}
}

// IsAuthenticated 返回当前请求是否携带有效 JWT
func IsAuthenticated(c *gin.Context) bool {
	return c.GetBool(contextAuthenticatedKey)
}

func bearerToken(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func setClaims(c *gin.Context, claims *Claims) {
	if claims == nil {
		return
	}
	c.Set(contextAuthenticatedKey, true)
	c.Set("username", claims.Username)
	c.Set("role", claims.Role)
}
