package go_redis_tokenbucket

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

// 使用Go的embed功能嵌入Lua脚本文件
//
//go:embed scripts/*.lua
var luaFS embed.FS

// TokenBucketLimiter 基于Redis的令牌桶限流器
type TokenBucketLimiter struct {
	client   *redis.Client // Redis客户端实例
	script   *redis.Script // 预加载的Lua脚本对象
	rate     time.Duration // 令牌生成速率（每个令牌所需时间）
	capacity int64         // 桶容量（最大令牌数）
	expire   time.Duration // Redis Key的过期时间
}

// LoadScript 预加载Lua脚本到Redis服务器
// ctx: 上下文，用于控制超时和取消
// client: Redis客户端实例
// 返回值:
//
//	*redis.Script - 加载成功的脚本对象
//	error - 加载过程中的错误
func LoadScript(ctx context.Context, client *redis.Client) (*redis.Script, error) {
	// 从嵌入的文件系统中读取Lua脚本内容
	scriptContent, err := luaFS.ReadFile("scripts/token_bucket.lua")
	if err != nil {
		// 文件读取失败，返回错误
		return nil, fmt.Errorf("failed to read Lua script: %w", err)
	}

	// 创建Redis脚本对象
	script := redis.NewScript(string(scriptContent))

	// 预加载脚本到Redis服务器（使用SCRIPT LOAD命令）
	// 这确保脚本在Redis中可用，后续可以使用SHA1哈希值执行
	if _, err := script.Load(ctx, client).Result(); err != nil {
		// 脚本加载失败，返回错误
		return nil, fmt.Errorf("failed to load redis script: %w", err)
	}

	// 返回加载成功的脚本对象
	return script, nil
}

// NewTokenBucketLimiter 创建并初始化令牌桶限流器
// client: Redis客户端实例
// rate: 令牌生成速率（每个令牌所需时间）
// capacity: 桶容量（最大令牌数）
// 返回值:
//
//	*TokenBucketLimiter - 初始化后的限流器实例
//	error - 创建过程中的错误
func NewTokenBucketLimiter(client *redis.Client, rate time.Duration, capacity int64) (*TokenBucketLimiter, error) {
	// 参数校验
	if client == nil {
		return nil, errors.New("redis client cannot be nil")
	}
	if rate <= 0 {
		return nil, errors.New("rate must be positive")
	}
	if capacity <= 0 {
		return nil, errors.New("capacity must be positive")
	}

	// 计算默认过期时间：桶填满所需时间的2倍
	expire := 2 * time.Duration(capacity) * rate
	// 确保最小过期时间为1秒
	if expire < time.Second {
		expire = time.Second
	}

	// 预加载Lua脚本（使用后台上下文）
	script, err := LoadScript(context.Background(), client)
	if err != nil {
		return nil, err
	}

	// 创建并返回限流器实例
	return &TokenBucketLimiter{
		client:   client,   // Redis客户端
		script:   script,   // 预加载的脚本
		rate:     rate,     // 令牌生成速率
		capacity: capacity, // 桶容量
		expire:   expire,   // Key过期时间
	}, nil
}

// Allow 尝试获取指定数量的令牌
// ctx: 上下文，用于控制超时和取消
// key: Redis中的键名（用于区分不同的限流器）
// n: 请求的令牌数量
// 返回值:
//
//	bool - true表示允许请求（获取到足够令牌）
//	error - 执行过程中的错误
func (l *TokenBucketLimiter) Allow(ctx context.Context, key string, n int64) (bool, error) {
	// 参数校验：请求令牌数必须在合理范围内
	if n <= 0 || n > l.capacity {
		// 无效请求（0或负数，或超过桶容量）
		return false, nil
	}

	// 获取当前时间（纳秒级）
	currentTime := time.Now().UnixNano()

	// 执行Lua脚本（带重试机制）
	result, err := l.script.Run(ctx, l.client, []string{key}, // KEYS参数
		l.rate.Nanoseconds(),    // 令牌生成速率 (纳秒/令牌)
		l.capacity,              // 桶容量
		n,                       // 请求令牌数
		l.expire.Milliseconds(), // Key过期时间(毫秒),
		currentTime,             // 当前时间(纳秒)
	).Int64() // 将结果转换为int64类型

	// 处理脚本执行错误
	if err != nil {
		return false, fmt.Errorf("script execution failed: %w", err)
	}

	// Lua脚本返回1表示允许，0表示拒绝
	return result == 1, nil
}

// SetExpiration 自定义Key过期时间
// d: 新的过期时间
func (l *TokenBucketLimiter) SetExpiration(d time.Duration) {
	l.expire = d
}

// Stats 返回限流器的当前配置状态
// 返回值: map[string]interface{} - 包含配置信息的键值对
func (l *TokenBucketLimiter) Stats() map[string]interface{} {
	return map[string]interface{}{
		"rate":     l.rate,     // 令牌生成速率
		"capacity": l.capacity, // 桶容量
		"expire":   l.expire,   // Key过期时间
	}
}
