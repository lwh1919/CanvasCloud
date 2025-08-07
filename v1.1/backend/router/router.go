package router

import (
	_ "backend/docs"
	"backend/internal/manager/websocket"
	"backend/internal/midwares"
	"backend/router/v1"
	"fmt"
	"github.com/gin-contrib/cors" // 修复导入路径错误
	"github.com/gin-gonic/gin"

	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Setup(mode string) *gin.Engine {
	if mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	//r.Use(logger.GinLogger(), logger.GinRecovery(true))
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// 使用纳秒精度
		latency := param.Latency.Nanoseconds()

		return fmt.Sprintf("[%s] %s %s %d %s %d ns\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.Method,
			param.Path,
			param.StatusCode,
			param.ClientIP,
			latency,
		)
	}))
	//midwares.InitSession(r)
	//r.Use(midwares.SwaggerSessionDebug())

	// 修复CORS配置
	// router.go
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},                                                 // 允许的来源（前端地址）
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},           // 允许的 HTTP 方法
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"}, // 允许的请求头
		ExposeHeaders:    []string{"Content-Length", "Authorization"},                   // 允许暴露的响应头
		AllowCredentials: true,                                                          // 是否允许携带凭证（如 Cookies）
		AllowWildcard:    true,                                                          // 是否允许任何来源
	}))

	// 创建v1路由组
	v1Group := r.Group("/v1")
	// 注册v1版本所有路由
	v1.RegisterV1Routes(v1Group)

	// 单独注册websocket路由
	r.GET("/ws/picture/edit", midwares.JWTAuthMiddleware(), websocket.PictureEditHandShake)

	// Swagger文档路由
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	return r
}
