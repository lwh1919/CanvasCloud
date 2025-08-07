package midwares

import (
	"context"
	"fmt"
	"strings"

	"backend/pkg/jwt"
	"backend/pkg/redis"

	"github.com/gin-gonic/gin"
)

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Header获取Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "未提供认证令牌"})
			return
		}

		// 格式：Bearer <token>
		//parts := strings.Split(authHeader, " ")
		//if len(parts) != 2 || parts[0] != "Bearer" {
		//	c.AbortWithStatusJSON(401, gin.H{"error": "令牌格式错误"})
		//	return
		//}
		//
		//tokenString := parts[1]
		// 替换原有的 parts 处理逻辑
		authHeader = strings.TrimSpace(authHeader)
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "未提供认证令牌"})
			return
		}

		// 支持多种大小写的 "Bearer" 标识
		prefix := "bearer "
		if len(authHeader) < len(prefix) || !strings.EqualFold(authHeader[:len(prefix)], prefix) {
			c.AbortWithStatusJSON(401, gin.H{
				"error":  "令牌格式错误",
				"detail": "Authorization 头必须以 'Bearer ' 开头",
			})
			return
		}

		tokenString := strings.TrimSpace(authHeader[len(prefix):])

		claims, err := jwt.VerifyToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "无效令牌"})
			return
		}

		// 检查用户是否在黑名单
		if isUserBlacklisted(claims.UserID) {
			c.AbortWithStatusJSON(401, gin.H{"error": "用户已被删除"})
			return
		}

		// 将用户ID存入上下文
		c.Set("jwtClaims", claims)
		c.Next()
	}
}

// 检查用户是否在黑名单
func isUserBlacklisted(userID uint64) bool {
	blacklistKey := fmt.Sprintf("jwt:blacklist:%d", userID)
	exists, err := redis.GetRedisClient().Exists(context.Background(), blacklistKey).Result()
	return err == nil && exists > 0
}
