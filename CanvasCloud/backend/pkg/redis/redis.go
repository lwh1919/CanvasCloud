package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"web_app2/config"
	"web_app2/pkg/redlock"
)

var redisClient *redis.Client

func Init(cfg *config.RedisConfig) error {
	//创建配置
	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), // 组合主机和端口
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize, // 可选：连接池大小
	})
	//连接测试
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		return err
	}
	//初始化分布式锁
	redlock.InitRedSync(redisClient)
	return nil
}

// 不暴露redis
func Close() {
	_ = redisClient.Close()
}
func GetRedisClient() *redis.Client {
	return redisClient
}

func IsNilErr(err error) bool {
	return err == redis.Nil
}
