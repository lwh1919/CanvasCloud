package main

import (
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
	"web_app2/config"
	"web_app2/internal/controller"
	"web_app2/internal/manager/websocket"
	"web_app2/internal/midwares"
	"web_app2/logger"
	"web_app2/pkg/cache"
	"web_app2/pkg/mysql"
	"web_app2/pkg/redis"
	"web_app2/pkg/snowflake"
	"web_app2/pkg/tcos"
	"web_app2/router"
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

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name gsessionid

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/

func main() {
	//1加载配置C:\Users\linweihao\Desktop\CanvasCloud\backend\config\config.yaml
	if err := config.Init(); err != nil {
		log.Fatalf("init setting  err: %v\n", err)
		return
	}
	//2. 初始化日志（使用配置中的设置）
	if err := logger.Init(config.Conf.LogConfig, config.Conf.Mode); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer zap.L().Sync()

	// 3. 雪花ID初始化（放在最前面，因为其他服务可能依赖ID生成）
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
	//8，初始化各个controller层的全局变量
	controller.Init()
	websocket.Init()
	//初始化中间件（session）
	midwares.Init()
	//注册路由
	r := router.Setup(config.Conf.Mode)

	zap.L().Info("应用启动成功",
		zap.String("version", "1.0.0"),
		zap.String("go_version", runtime.Version()))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", viper.GetInt("port")),
		Handler: r,
	}
	//启动服务（优雅关机）
	go func() {
		// 开启一个goroutine启动服务
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// 等待中断信号来优雅地关闭服务器，为关闭服务器操作设置一个5秒的超时
	quit := make(chan os.Signal, 1) // 创建一个接收信号的通道

	// signal.Notify设置 quit 收到 的 syscall.SIGINT或syscall.SIGTERM
	//syscall.SIGINT：Ctrl+C 中断信号
	//syscall.SIGTERM：kill 命令默认发送的信号
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM) // 此处不会阻塞
	<-quit                                               // 阻塞在此，当接收到上述两种信号时才会往下执行
	zap.L().Info("Shutdown Server ...")

	// 创建一个5秒超时的context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 5秒内优雅关闭服务（将未处理完的请求处理完再关闭服务），超过5秒就超时退出
	if err := srv.Shutdown(ctx); err != nil {
		zap.L().Fatal("Server Shutdown: ", zap.Error(err))
	}

	zap.L().Info("Server exiting...")
}
