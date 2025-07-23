package casbin

import (
	"bufio"
	"fmt"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3" // Casbin的GORM适配器
	"gorm.io/gorm"                                  // GORM ORM库
	"strings"

	// 引入 embed 包（必须导入），用于嵌入文件到二进制中
	_ "embed" // Go 1.16+ 嵌入式文件特性
)

// 下面的指令将模型和策略文件嵌入到编译后的二进制文件中
// 使用 //go:embed 指令在编译时将文件内容嵌入变量

// 嵌入RBAC模型配置文件
//
//go:embed rbac_model.conf
var embeddedRBACModelConf string // 存储模型配置内容// ← 编译时会被替换为文件内容

// 嵌入RBAC策略文件（CSV格式）
//
//go:embed rbac_policy.csv
var embeddedRBACPolicyCsv string // 存储策略内容// ← 编译时会被替换为文件内容

// CasbinMethod 结构体封装Casbin的核心组件
type CasbinMethod struct {
	Enforcer *casbin.Enforcer     // Casbin执行器，负责权限验证
	Adapter  *gormadapter.Adapter // GORM适配器，连接数据库
}

// Casbin 全局Casbin实例，作为单例模式使用
var Casbin *CasbinMethod

// LoadCasbinMethod 提供全局Casbin实例的访问点
func LoadCasbinMethod() *CasbinMethod {
	return Casbin
}

// InitCasbinGorm 初始化Casbin的Gorm适配器，并从嵌入的文件加载模型和策略
// 参数: db - GORM数据库连接
// 返回值: *CasbinMethod - 初始化后的Casbin实例指针, error - 错误信息
func InitCasbinGorm(db *gorm.DB) (*CasbinMethod, error) {
	// 1. 创建GORM适配器 - 将Casbin策略存储在数据库
	a, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("创建GORM适配器失败: %v", err)
	}

	// 2. 从嵌入的字符串创建Casbin模型
	m, err := model.NewModelFromString(embeddedRBACModelConf)
	if err != nil {
		return nil, fmt.Errorf("解析Casbin模型失败: %v", err)
	}

	// 3. 初始化Casbin执行器
	enforcer, err := casbin.NewEnforcer(m, a) // 组合模型和适配器
	if err != nil {
		return nil, fmt.Errorf("初始化执行器失败: %v", err)
	}

	// 4. 从嵌入的CSV字符串加载策略到Casbin
	if err := loadCsvPolicy(enforcer, embeddedRBACPolicyCsv); err != nil {
		return nil, fmt.Errorf("加载CSV策略失败: %v", err)
	}

	// 5. 创建并设置全局Casbin实例
	Casbin = &CasbinMethod{
		Enforcer: enforcer,
		Adapter:  a,
	}

	return Casbin, nil
}

// loadCsvPolicy 从CSV字符串加载策略到Casbin执行器
// 参数: e - Casbin执行器, csvContent - CSV内容字符串
// 返回值: error - 错误信息
func loadCsvPolicy(e *casbin.Enforcer, csvContent string) error {
	// 创建字符串扫描器处理CSV内容
	scanner := bufio.NewScanner(strings.NewReader(csvContent))

	for scanner.Scan() {
		// 读取每一行并清理
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 按逗号分割字段
		parts := strings.Split(line, ",")

		// 清理每个字段的空格
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		// 根据策略类型处理
		switch parts[0] {
		case "p": // 权限策略 (p, 角色, 资源, 操作)
			if len(parts) < 4 {
				continue // 字段不足时跳过
			}
			// 添加权限策略
			_, _ = e.AddPolicy(parts[1], parts[2], parts[3])
		case "g": // 分组策略 (g, 用户, 角色, 域)
			if len(parts) == 4 {
				// 添加角色分组策略
				_, _ = e.AddGroupingPolicy(parts[1], parts[2], parts[3])
			}
		}
	}

	// 构建角色链接关系（处理角色继承）
	e.BuildRoleLinks()

	// 将策略保存到数据库适配器
	return e.SavePolicy()
}

// UpdateUserRoleInDomain 更新用户在指定域的角色
// 参数:
//
//	c - Casbin实例, userID - 用户ID, role - 新角色, domain - 域(如空间ID或全局)
//
// 返回值: error - 错误信息
func UpdateUserRoleInDomain(c *CasbinMethod, userID uint64, role string, domain string) error {
	// 生成Casbin用户标识 (格式: "user_123")
	sub := fmt.Sprintf("user_%d", userID)

	// 1. 获取用户在指定域的旧角色列表
	oldRoles := c.Enforcer.GetRolesForUserInDomain(sub, domain)

	// 2. 移除用户在指定域的所有旧角色
	for _, oldRole := range oldRoles {
		_, err := c.Enforcer.DeleteRoleForUserInDomain(sub, oldRole, domain)
		if err != nil {
			return fmt.Errorf("删除旧角色失败: %v", err)
		}
	}

	// 3. 添加新角色到用户
	ok, err := c.Enforcer.AddRoleForUserInDomain(sub, role, domain)
	if err != nil || !ok {
		return fmt.Errorf("添加角色失败: %v", err)
	}

	// 4. 重建角色链接（处理角色继承关系）
	c.Enforcer.BuildRoleLinks()

	// 5. 持久化变更到存储后端，通过gorm适配器存到mysql
	err = c.Enforcer.SavePolicy()
	if err != nil {
		return fmt.Errorf("持久化角色策略失败: %v", err)
	}

	return nil
}
