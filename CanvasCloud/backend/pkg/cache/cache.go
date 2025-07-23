package cache

import (
	"github.com/dgraph-io/ristretto"
	"log"
)

// 本地缓存单例，使用ristretto实现，适用于单机缓存
// 缓存高频数据
// 全局缓存变量
var LocalCache *ristretto.Cache

func Init() error {
	var err error
	LocalCache, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6,       // 100 万个计数器，适用于 100K ~ 500K 的缓存项
		MaxCost:     512 << 20, // 512MB 存储成本
		BufferItems: 64,        //并发优化
	})
	if err != nil {
		return err
	}
	log.Println("本地缓存初始化成功！")
	return nil
}

func GetCache() *ristretto.Cache {
	return LocalCache
}
