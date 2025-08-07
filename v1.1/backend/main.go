package main

import (
	"backend/config"
	"backend/internal/controller"
	"backend/internal/manager/websocket"
	"backend/internal/service"
	"backend/logger"
	"backend/pkg/cache"
	"backend/pkg/casbin" // 确保导入casbin包
	"backend/pkg/mq"
	"backend/pkg/mysql"
	"backend/pkg/redis"
	"backend/pkg/snowflake"
	"backend/pkg/tcos"
	"backend/router"
	"context"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// @title CanvasCloud
// @version 1.0
// @description CanvasCloud
// @termsOfService 无服务条款
// @contact.name lwhhhh
// @contact.url 无
// @contact.email lwh24621@qq.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8001
// @BasePath
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT 认证格式: Bearer <token>
// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
func main() {
	// 1. 加载配置
	if err := config.Init(); err != nil {
		log.Fatalf("init setting  err: %v\n", err)
		return
	}

	// 2. 初始化日志
	if err := logger.Init(config.Conf.LogConfig, config.Conf.Mode); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer zap.L().Sync()

	// 3. 雪花ID初始化
	if err := snowflake.Init("2020-01-01", 1); err != nil {
		zap.L().Fatal("雪花ID生成器初始化失败", zap.Error(err))
	}
	zap.L().Info("雪花ID生成器初始化成功")

	// 4. 初始化MySQL
	if err := mysql.Init(config.Conf.MySQLConfig); err != nil {
		zap.L().Fatal("MySQL初始化失败", zap.Error(err))
	}
	defer mysql.Close()
	zap.L().Info("MySQL初始化成功")

	// 5. 初始化Redis
	if err := redis.Init(config.Conf.RedisConfig); err != nil {
		zap.L().Fatal("Redis初始化失败", zap.Error(err))
	}
	defer redis.Close()
	zap.L().Info("Redis初始化成功")

	// 6. 初始化缓存
	if err := cache.Init(); err != nil {
		zap.L().Fatal("缓存初始化失败", zap.Error(err))
	}
	zap.L().Info("缓存初始化成功")

	// 7. 初始化腾讯云COS
	if err := tcos.Init(); err != nil {
		zap.L().Fatal("COS初始化失败", zap.Error(err))
	}
	zap.L().Info("COS初始化成功")

	// 8. 初始化Casbin (必须在MySQL之后)
	if _, err := casbin.InitCasbinGorm(mysql.LoadDB()); err != nil {

		zap.L().Fatal("Casbin初始化失败", zap.Error(err))
	}
	zap.L().Info("Casbin初始化成功")

	// 9. 初始化各个controller层的全局变量
	controller.Init()
	websocket.Init()
	if err := mq.InitMq(); err != nil {
		zap.L().Fatal("mq初始化失败", zap.Error(err))
	}
	zap.L().Info("mq启动成功")
	// 10. 启动mq
	go func() {
		service.OutPaintingBackgroundService()
	}()

	// 11. 注册路由
	r := router.Setup(config.Conf.Mode)

	zap.L().Info("应用启动成功",
		zap.String("version", "1.0.0"),
		zap.String("go_version", runtime.Version()))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", viper.GetInt("port")),
		Handler: r,
	}

	// 启动服务（优雅关机）
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zap.L().Info("Shutdown Server ...")

	// 先关闭Casbin异步系统
	casbin.Shutdown()
	zap.L().Info("Casbin异步系统已关闭")

	// 原有优雅关闭逻辑
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		zap.L().Fatal("Server Shutdown: ", zap.Error(err))
	}

	zap.L().Info("Server exiting...")
}
