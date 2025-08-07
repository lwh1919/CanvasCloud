package argon2

import (
	"testing"
)

// 性能测试函数
func BenchmarkArgon2Hash(b *testing.B) {
	password := "lwh123"
	salt := "salt"
	// 重置计时器，忽略上一行耗时
	b.ResetTimer()

	// 运行 b.N 次，测试生成哈希值的速度
	for i := 0; i < b.N; i++ {
		GetEncryptString(password, salt) // 只测量此部分
	}
}
