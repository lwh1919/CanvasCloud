// 声明当前包名（注意：与导入的库同名但可区分）
package snowflake

import (
	"errors"
	"fmt"
	"time"

	"github.com/sony/sonyflake"
)

// 全局 Sonyflake 实例
var flake *sonyflake.Sonyflake

// Init 函数：初始化 Sonyflake ID 生成器
// 参数说明:
//
//	startTime string  - 起始时间字符串，格式必须为"2006-01-02"
//	machineID uint16  - 机器标识符，范围必须是 0-65535
//
// 返回值:
//
//	error - 初始化成功返回 nil，失败返回具体错误
func Init(startTime string, machineID uint16) error {
	// 步骤1：解析字符串时间
	st, err := time.Parse("2006-01-02", startTime)
	if err != nil {
		return fmt.Errorf("解析起始时间失败: %w", err)
	}

	// 步骤2：创建 Sonyflake 配置
	settings := sonyflake.Settings{
		StartTime: st,
		MachineID: func() (uint16, error) {
			return machineID, nil
		},
	}

	// 步骤3：创建 Sonyflake 实例
	flake = sonyflake.NewSonyflake(settings)
	if flake == nil {
		return errors.New("创建 Sonyflake 实例失败")
	}

	return nil
}

// GenID 生成唯一 ID
func GenID() (uint64, error) {
	if flake == nil {
		return 0, errors.New("Sonyflake 实例未初始化")
	}
	return flake.NextID()
}

// GenIDString 生成字符串格式的 ID
func GenIDString() (string, error) {
	id, err := GenID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", id), nil
}

// GenIDWithPrefix 生成带前缀的字符串 ID
func GenIDWithPrefix(prefix string) (string, error) {
	id, err := GenID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%d", prefix, id), nil
}
