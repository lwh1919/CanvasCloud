package midwares

import (
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"web_app2/config"
	"web_app2/internal/model/entity"

	"github.com/gin-contrib/sessions"
	redisSession "github.com/gin-contrib/sessions/redis" // 使用别名解决冲突
	"github.com/gin-gonic/gin"
	redigo "github.com/gomodule/redigo/redis" // 使用正确的redigo包
)

// 注册用户结构体（必须）
func Init() {
	gob.Register(entity.User{})
}

// 主 Redis 操作
// // go-redis 风格（直接高效）
// rdb.Set(ctx, "key", "value", 0)
// Session 存储
// // redigo 三步法（遵循 Gin 中间件规范）
// pool := createRedigoPool()
// store := NewStoreWithPool(pool)
// store.Options({...})

func InitSession(r *gin.Engine) {
	cfg := config.LoadConfig()
	address := fmt.Sprintf("%s:%d", cfg.RedisConfig.Host, cfg.RedisConfig.Port)
	secretKey := getSessionSecret()

	log.Printf("Connecting Redis: %s | Password length: %d", address, len(cfg.RedisConfig.Password))

	// 1. 创建redigo连接池（含智能认证）
	pool := createRedisPool(address, cfg.RedisConfig.Password)

	// 2. 创建会话存储
	store, err := redisSession.NewStoreWithPool(pool, []byte(secretKey))
	if err != nil {
		log.Fatalf("Session store init failed: %v", err)
	}

	// 3. 配置会话选项
	store.Options(sessions.Options{
		Path:     "/",
		Domain:   getDomain(),
		MaxAge:   3600,
		HttpOnly: false,
		Secure:   isHTTPS(),
		SameSite: getSameSite(),
	})

	r.Use(sessions.Sessions("gsessionid", store)) // gsessionid改为小写
	log.Println("Redis session initialized!")
}

// 创建redigo连接池（解决所有编译错误）
func createRedisPool(address, password string) *redigo.Pool {
	return &redigo.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redigo.Conn, error) {
			conn, err := redigo.Dial("tcp", address)
			if err != nil {
				return nil, err
			}

			if password == "" {
				return conn, nil
			}

			// 智能认证流程
			//conn.Do()是一个核心方法，用于向 Redis 服务器发送命令并获取返回结果
			if _, err := conn.Do("AUTH", "default", password); err == nil {
				log.Println("ACL auth success")
				return conn, nil
			}

			if _, err := conn.Do("AUTH", password); err == nil {
				log.Println("Legacy auth success")
				return conn, nil
			}

			// 修正错误字符串
			conn.Close()
			return nil, fmt.Errorf("redis auth failed") // 改为小写，去掉标点
		},
		TestOnBorrow: func(c redigo.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

// 获取会话加密密钥（优先环境变量）
func getSessionSecret() string {
	if secret := os.Getenv("SESSION_SECRET"); secret != "" {
		return secret
	}
	return "CanvasCloudDefaultSessionSecret"
}

// 获取域名配置（环境变量优先）
func getDomain() string {
	if domain := os.Getenv("APP_DOMAIN"); domain != "" {
		return domain
	}
	return "localhost"
}

// 检查是否启用HTTPS
func isHTTPS() bool {
	return os.Getenv("HTTPS_ENABLED") == "true"
}

// 动态设置SameSite策略
func getSameSite() http.SameSite {
	if gin.Mode() == gin.ReleaseMode {
		return http.SameSiteLaxMode
	}
	return http.SameSiteDefaultMode
}
