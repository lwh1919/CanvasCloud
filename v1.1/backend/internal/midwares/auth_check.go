package midwares

import (
	"backend/internal/common"
	"backend/internal/consts"
	"backend/internal/ecode"
	"backend/internal/service"
	"backend/pkg/redis"
	"context"
	"github.com/gin-gonic/gin"
)

// 权限校验中间件，只有拥有权限的用户才允许执行接下来的服务
func AuthCheck(mustRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userService := service.NewUserService()
		//获取当前登录对象
		loginUser, err := userService.GetLoginUser(c)
		if err != nil {
			//未登录或者出错
			common.BaseResponse(c, nil, err.Msg, err.Code)
			c.Abort()
			return
		}
		if mustRole == "" {
			//不需要权限，放行
			c.Next()
			return
		}

		//需要权限：
		//校验用户角色，管理员权限只能管理员用，普通权限管理员和用户可用
		if (mustRole == consts.ADMIN_ROLE && loginUser.UserRole != consts.ADMIN_ROLE) || loginUser.UserRole == "" {
			common.BaseResponse(c, nil, "无权限", ecode.NO_AUTH_ERROR)
			c.Abort()
			return
		}
		//权限通过，放行
		c.Next()
	}
}
func isTokenBlacklisted(tokenString string) bool {
	result, err := redis.GetRedisClient().Exists(
		context.Background(),
		"jwt:blacklist:"+tokenString,
	).Result()

	return err == nil && result > 0
}
