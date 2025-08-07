package argon2

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
)

// 安全参数配置（可后期调优）
const (
	time    uint32 = 2        // 迭代次数（根据服务器压力调整）
	memory  uint32 = 8 * 1024 // 内存使用量（8MB）
	threads uint8  = 4        // 并行度
	keyLen  uint32 = 32       // 生成哈希值长度
	saltLen int    = 16       // 盐值长度（必须使用16字节！）
)

// GenerateSecureSalt 生成安全随机盐值
func GenerateSecureSalt() (string, error) {
	saltBytes := make([]byte, saltLen)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", errors.New("盐值生成失败")
	}
	return base64.RawStdEncoding.EncodeToString(saltBytes), nil
}

// GetEncryptPassword 安全加密方法
func GetEncryptPassword(password string) (string, error) {
	salt, err := GenerateSecureSalt()
	if err != nil {
		return "", err
	}
	return GetEncryptString(password, salt), nil
}

// GetEncryptString 兼容方法（支持新老格式）
func GetEncryptString(value, salt string) string {
	// 安全标准化盐值
	var saltBytes []byte
	if decoded, err := base64.RawStdEncoding.DecodeString(salt); err == nil {
		saltBytes = decoded
	} else {
		saltBytes = []byte(salt)
	}

	// 生成哈希值
	hashed := argon2.IDKey([]byte(value), saltBytes, time, memory, threads, keyLen)

	// 新格式：$算法$参数$盐$哈希
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory, time, threads,
		base64.RawStdEncoding.EncodeToString(saltBytes),
		base64.RawStdEncoding.EncodeToString(hashed))
}

// IsLegacyPassword 检查是否为旧格式密码
func IsLegacyPassword(hash string) bool {
	return !strings.HasPrefix(hash, "$argon2id")
}

// VerifyPassword 统一验证方法
func VerifyPassword(inputPassword, storedHash string) (bool, error) {
	// 新格式验证
	if strings.HasPrefix(storedHash, "$argon2id") {
		return verifyNewFormat(inputPassword, storedHash)
	}

	// 旧格式验证（仅迁移期使用）
	return verifyOldFormat(inputPassword, storedHash), nil
}

// 新格式验证
func verifyNewFormat(inputPassword, storedHash string) (bool, error) {
	parts := strings.Split(storedHash, "$")
	if len(parts) < 6 {
		return false, errors.New("无效的哈希格式")
	}

	// 解析盐值
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errors.New("无效的盐值编码")
	}

	// 解析存储的哈希值
	hashBytes, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, errors.New("无效的哈希编码")
	}

	// 重新计算哈希
	calculatedHash := argon2.IDKey(
		[]byte(inputPassword), salt, time, memory, threads, keyLen)

	// 安全比较
	return subtle.ConstantTimeCompare(calculatedHash, hashBytes) == 1, nil
}

// 旧格式验证（迁移期兼容）
func verifyOldFormat(inputPassword, storedHash string) bool {
	salt := inputPassword[:min(5, len(inputPassword))]

	calculatedHash := argon2.IDKey(
		[]byte(inputPassword), []byte(salt), time, memory, threads, keyLen)

	calculatedHashStr := base64.RawStdEncoding.EncodeToString(calculatedHash)

	return calculatedHashStr == storedHash
}
