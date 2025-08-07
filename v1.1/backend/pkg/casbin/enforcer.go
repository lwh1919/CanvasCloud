package casbin

import (
	"bufio"
	"fmt"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/panjf2000/ants/v2"
	"gorm.io/gorm"
	"log"
	"strings"
	"sync"
	"time"

	_ "embed"
)

// 嵌入RBAC模型配置文件
//
//go:embed rbac_model.conf
var embeddedRBACModelConf string

// 嵌入RBAC策略文件（CSV格式）
//
//go:embed rbac_policy.csv
var embeddedRBACPolicyCsv string

// CasbinMethod 结构体封装Casbin的核心组件
type CasbinMethod struct {
	Enforcer *casbin.Enforcer     // Casbin执行器，负责权限验证
	Adapter  *gormadapter.Adapter // GORM适配器，连接数据库
}

const (
	workerPoolSize = 100
	taskQueueSize  = 1000
	batchSize      = 50
	batchTimeout   = 100 * time.Millisecond
)

var (
	taskPool   *ants.Pool
	taskChan   chan RoleTask
	batchMutex sync.Mutex
	batchQueue []RoleTask
	stopChan   chan struct{} //用于等待一组 goroutine 完成工作
	wg         sync.WaitGroup
)

type RoleTask struct {
	UserID uint64
	Role   string
	Domain string
}

var CasbinInstance *CasbinMethod

func LoadCasbinMethod() *CasbinMethod {
	return CasbinInstance
}

func InitCasbinGorm(db *gorm.DB) (*CasbinMethod, error) {
	// 1. 创建适配器 (不使用WithConfig)
	adapter, merr := gormadapter.NewAdapterByDB(db)
	if merr != nil {
		return nil, merr
	}
	// 2. 创建模型
	m, merr := model.NewModelFromString(embeddedRBACModelConf)
	if merr != nil {
		return nil, merr
	}
	// 3. 初始化执行器 (使用NewEnforcer代替NewCachedEnforcer)
	enforcer, merr := casbin.NewEnforcer(m, adapter)
	if merr != nil {
		return nil, merr
	}

	// 4. 加载策略
	loadCsvPolicy(enforcer, embeddedRBACPolicyCsv)

	// 5. 创建全局实例
	CasbinInstance = &CasbinMethod{
		Enforcer: enforcer,
		Adapter:  adapter,
	}

	// 6. 初始化异步系统
	initAsyncSystem()

	return CasbinInstance, nil
}

func initAsyncSystem() {
	var err error

	// 1. 创建任务通道
	taskChan = make(chan RoleTask, taskQueueSize)

	// 2. 初始化协程池
	taskPool, err = ants.NewPool(workerPoolSize, ants.WithPreAlloc(true))
	if err != nil {
		panic(fmt.Sprintf("创建协程池失败: %v", err))
	}

	// 3. 启动批量处理器
	wg.Add(1)
	go batchProcessor()

	// 4. 优雅关闭通道
	stopChan = make(chan struct{})
}

func batchProcessor() {
	defer wg.Done()

	ticker := time.NewTicker(batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case task := <-taskChan:
			processTask(task)
		case <-ticker.C:
			flushBatch()
		case <-stopChan:
			flushBatch() // 退出前处理剩余任务
			return
		}
	}
}

func processTask(task RoleTask) {
	batchMutex.Lock()
	defer batchMutex.Unlock()

	batchQueue = append(batchQueue, task)
	if len(batchQueue) >= batchSize {
		flushBatch()
	}
}

func flushBatch() {
	batchMutex.Lock()
	defer batchMutex.Unlock()

	if len(batchQueue) == 0 {
		return
	}

	// 复制当前批次
	tasks := make([]RoleTask, len(batchQueue))
	copy(tasks, batchQueue)
	batchQueue = nil

	// 提交到协程池
	taskPool.Submit(func() {
		processBatch(tasks)
	})
}
func processBatch(tasks []RoleTask) {
	startTime := time.Now()
	log.Printf("🚀 开始批量处理角色分配: 数量=%d", len(tasks))

	rules := make([][]string, 0, len(tasks))
	for _, task := range tasks {
		rules = append(rules, []string{
			"g",
			fmt.Sprintf("user_%d", task.UserID),
			task.Role,
			task.Domain,
		})
	}

	// 原子性批量操作
	if _, err := CasbinInstance.Enforcer.AddGroupingPolicies(rules); err != nil {
		log.Printf("⚠️ 批量添加角色失败: 数量=%d, 错误=%v", len(tasks), err)
	} else {
		log.Printf("✅ 批量添加角色成功: 数量=%d, 耗时=%v", len(tasks), time.Since(startTime))
		CasbinInstance.Enforcer.BuildRoleLinks()
	}
}

// 异步提交角色分配任务
func UpdateUserRoleInDomain(userID uint64, role string, domain string) error {
	// 提交到异步系统
	submitRoleTask(RoleTask{
		UserID: userID,
		Role:   role,
		Domain: domain,
	})
	return nil
}

// 提交任务（带队列满的兜底策略）
func submitRoleTask(task RoleTask) {
	select {
	case taskChan <- task: // 正常入队
	default:
		// 队列满时直接执行
		taskPool.Submit(func() {
			processSingleTask(task)
		})
	}
}

func processSingleTask(task RoleTask) {
	_, err := CasbinInstance.Enforcer.AddGroupingPolicy(
		"g",
		fmt.Sprintf("user_%d", task.UserID),
		task.Role,
		task.Domain,
	)
	if err != nil {
		log.Printf("⚠️ 单任务添加失败: %v", err)
	} else {
		CasbinInstance.Enforcer.BuildRoleLinks()
	}
}

// 优雅关闭
func Shutdown() {
	close(stopChan)    // 通知批量处理器退出
	wg.Wait()          // 等待批量处理器退出
	taskPool.Release() // 释放协程池
	close(taskChan)    // 关闭任务通道
}

// 从CSV字符串加载策略到Casbin执行器
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
