# RabbitMQ + ants 协程池完整教程

## 概述

本项目采用 **RabbitMQ + ants协程池** 的架构模式，实现了高性能的异步任务处理系统。通过消息队列解耦和协程池并发控制，有效提升了系统的响应速度和并发处理能力。

## 架构图

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   用户请求       │───▶│   API接口        │───▶│   任务创建       │
│                 │    │                  │    │   数据库         │
└─────────────────┘    └──────────────────┘    └────────┬────────┘
                                                           │
                                                           │ 发布消息
                                                           ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   结果返回       │◀───│   任务处理       │◀───│   RabbitMQ      │
│                 │    │   ants协程池      │    │   消息队列       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## 1. RabbitMQ配置与初始化

### 1.1 配置文件 (config/config.go)

```go
type RabbitMQConfig struct {
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    UserName string `mapstructure:"username"`
    Password string `mapstructure:"password"`
}
```

### 1.2 常量定义 (internal/consts/chart_constant.go)

```go
const (
    MQExchangeName = "mrbi" // 交换机名称
    MQRoutingKey   = "mrbi" // 路由键名称

    // 外绘任务队列和消费者
    MQOutPaintingQueueName  = "out_painting_tasks"
    OutPaintingConsumerName = "outpainting_consumer"

    // 死信队列配置
    MQDeadLetterExchangeName = "dlx.exchange"    // 死信交换机名称
    MQDeadLetterQueueName    = "dlx.queue"       // 死信队列名称
    MQDeadLetterRoutingKey   = "dlx.routing.key" // 死信路由键
)
```

### 1.3 RabbitMQ连接池 (pkg/mq/rabbitmq.go)

#### 1.3.1 核心结构

```go
// 全局连接池实例
var connPool *ChannelPool

// ChannelPool 定义通道连接池结构
type ChannelPool struct {
    conn *amqp.Connection   // RabbitMQ 服务器连接
    pool chan *amqp.Channel // 缓冲通道，用于存储和管理可用通道
}
```

#### 1.3.2 初始化流程

```go
func InitMq() error {
    // 1. 加载配置
    cfg := config.LoadConfig().RabbitMQConfig
    
    // 2. 创建连接
    conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/",
        cfg.UserName, cfg.Password, cfg.Host, cfg.Port))
    
    // 3. 初始化连接池
    connPool = &ChannelPool{
        conn: conn,
        pool: make(chan *amqp.Channel, 6), // 缓冲通道容量为5
    }
    
    // 4. 声明基础设施
    // 4.1 声明死信交换机
    ch.ExchangeDeclare(consts.MQDeadLetterExchangeName, "direct", true, false, false, false, nil)
    
    // 4.2 声明死信队列
    ch.QueueDeclare(consts.MQDeadLetterQueueName, true, false, false, false, nil)
    
    // 4.3 绑定死信队列到死信交换机
    ch.QueueBind(consts.MQDeadLetterQueueName, consts.MQDeadLetterRoutingKey, 
                 consts.MQDeadLetterExchangeName, false, nil)
    
    // 5. 声明主队列（带死信参数）
    args := amqp.Table{
        "x-dead-letter-exchange":    consts.MQDeadLetterExchangeName,
        "x-dead-letter-routing-key": consts.MQDeadLetterRoutingKey,
        "x-message-ttl":             600000, // 10分钟TTL
    }
    
    // 5.1 声明交换机
    ch.ExchangeDeclare(consts.MQExchangeName, "direct", true, false, false, false, args)
    
    // 5.2 声明队列
    ch.QueueDeclare(consts.MQOutPaintingQueueName, true, false, false, false, nil)
    
    // 5.3 绑定队列到交换机
    ch.QueueBind(consts.MQOutPaintingQueueName, consts.MQRoutingKey, 
                 consts.MQExchangeName, false, nil)
    
    // 6. 预创建通道并放入连接池
    for i := 0; i < cap(connPool.pool); i++ {
        channel, err := conn.Channel()
        channel.Qos(20, 0, false) // 每次最多接收20条未确认消息
        connPool.pool <- channel
    }
    
    return nil
}
```

#### 1.3.3 消息发布

```go
func (connPool *ChannelPool) PublishMessage(message []byte) error {
    ch := <-connPool.pool
    defer func() { connPool.pool <- ch }()
    
    return ch.Publish(
        consts.MQExchangeName,
        consts.MQRoutingKey,
        false,
        false,
        amqp.Publishing{
            ContentType: "text/plain",
            Body:        message,
        },
    )
}
```

#### 1.3.4 死信队列消费者

```go
func StartDLXConsumer() {
    ch := GetChannel()
    defer ReleaseChannel(ch)
    
    msgs, err := ch.Consume(
        consts.MQDeadLetterQueueName,
        "dlx_consumer",
        false, // 手动ACK
        false, false, false, nil)
    
    go func() {
        for d := range msgs {
            log.Printf("死信告警: 任务 %s 进入DLQ, 原始路由键=%s, 错误原因=%v",
                d.Body, d.RoutingKey, d.Headers["x-death"])
            d.Ack(false)
        }
    }()
}
```

## 2. ants协程池配置

### 2.1 协程池初始化 (internal/service/iTask_service.go)

```go
var aiGenPool *ants.Pool
var aiGenPoolOnce sync.Once

// 保证只初始化一次协程池
func GetAiGenPool() *ants.Pool {
    aiGenPoolOnce.Do(func() {
        var err error
        aiGenPool, err = ants.NewPool(4, // 固定大小为4的协程池
            ants.WithMaxBlockingTasks(20),  // 最大阻塞任务数20
            ants.WithPreAlloc(true),        // 预分配内存
            ants.WithNonblocking(true))     // 非阻塞模式
        if err != nil {
            panic(fmt.Sprintf("创建协程池失败: %v", err))
        }
    })
    return aiGenPool
}
```

### 2.2 任务处理流程

```go
func OutPaintingBackgroundService() {
    log.Println("启动外绘背景服务...")
    aiPool := GetAiGenPool() // 获取协程池
    ch := mq.GetChannel()
    defer mq.ReleaseChannel(ch)
    
    // 注册RabbitMQ消费者
    msgs, err := ch.Consume(
        consts.MQOutPaintingQueueName,
        consts.OutPaintingConsumerName,
        false, // 手动ACK
        false, false, false, nil)
    
    // 启动死信队列消费者
    go mq.StartDLXConsumer()
    
    // 消息处理主循环
    for d := range msgs {
        taskId, _ := strconv.ParseUint(string(d.Body), 10, 64)
        
        // 提交任务到协程池
        taskProcessErr := aiPool.Submit(func() {
            // 实际的任务处理逻辑
            processTask(taskId, d)
        })
        
        if taskProcessErr != nil {
            log.Printf("[任务 %d] 任务提交到协程池失败: %v", taskId, taskProcessErr)
            d.Nack(false, true) // 重新入队
        }
    }
}
```

## 3. 完整业务流程

### 3.1 任务创建流程

```go
func (s *ITaskService) ProCreatePictureOutPaintingTask(req *iTask.TaskRequest, userId uint64) *ecode.ErrorWithCode {
    // 1. 创建任务记录
    task := entity.ITask{
        Name:           "",
        Prompt:         req.Prompt,
        OriginalPicUrl: req.ImageURL,
        ExpandParams:   "{}",
        Status:         consts.TaskStatusWait,
        UserID:         userId,
    }
    
    if err := s.ITaskRepo.Create(nil, &task); err != nil {
        return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
    }
    
    // 2. 发送MQ消息
    s.sendToMQ(&task)
    return nil
}

func (s *ITaskService) sendToMQ(task *entity.ITask) {
    pool := mq.GetChannelPool()
    message := []byte(strconv.FormatUint(task.ID, 10))
    
    if err := pool.PublishMessage(message); err != nil {
        log.Printf("MQ消息发送失败! 任务ID: %d, 错误: %v", task.ID, err)
        
        // 失败回滚任务状态
        updateMap := map[string]interface{}{
            "status":       consts.TaskStatusWait,
            "exec_message": "MQ发送失败，等待重试",
        }
        _ = s.ITaskRepo.UpdateByMap(nil, task.ID, updateMap)
    } else {
        log.Printf("MQ消息成功发送! 任务ID: %d", task.ID)
    }
}
```

### 3.2 任务处理流程

```go
func processTask(taskId uint64, d amqp.Delivery) {
    log.Printf("[任务 %d] 任务开始处理", taskId)
    
    // 1. 获取任务信息
    taskSvc := NewITaskService()
    iTask, err := taskSvc.ITaskRepo.FindById(nil, taskId)
    
    // 2. 更新任务状态为运行中
    updateMap := map[string]interface{}{
        "status":       consts.TaskStatusRunning,
        "exec_message": "正在执行",
    }
    taskSvc.ITaskRepo.UpdateByMap(nil, iTask.ID, updateMap)
    
    // 3. 调用AI处理
    result, err := taskSvc.processTask(iTask)
    if err != nil {
        if isRecoverableError(err) {
            d.Nack(false, true)  // 可恢复错误，重新入队
        } else {
            d.Nack(false, false) // 不可恢复错误，进入死信队列
        }
        return
    }
    
    // 4. 更新任务状态为成功
    updateMap = map[string]interface{}{
        "status":           consts.TaskStatusSucceed,
        "exec_message":     "执行成功",
        "expanded_pic_url": result.DirectURL,
        "ai_recap":         result.Analysis,
    }
    taskSvc.ITaskRepo.UpdateByMap(nil, iTask.ID, updateMap)
    
    // 5. 确认消息处理完成
    d.Ack(false)
}
```

### 3.3 错误处理机制

```go
// 判断错误是否可恢复
func isRecoverableError(err error) bool {
    errMsg := err.Error()
    
    // 可恢复错误
    switch {
    case strings.Contains(errMsg, "connection refused"),
         strings.Contains(errMsg, "timeout"),
         strings.Contains(errMsg, "API返回错误: 5"):
        return true
    }
    
    // 不可恢复错误
    switch {
    case strings.Contains(errMsg, "API返回错误: 4"),
         strings.Contains(errMsg, "json.Marshal"),
         strings.Contains(errMsg, "AI返回结果为空"):
        return false
    }
    
    return false // 保守策略
}
```

## 4. 性能优化要点

### 4.1 连接池优化
- **连接复用**：使用连接池避免频繁创建/销毁连接
- **通道池化**：预创建通道，减少创建开销
- **QoS控制**：限制每个消费者处理的消息数量，防止内存溢出

### 4.2 协程池优化
- **固定大小**：4个协程，避免过多goroutine
- **预分配内存**：减少GC压力
- **非阻塞模式**：防止任务堆积
- **最大阻塞限制**：20个阻塞任务上限

### 4.3 消息队列优化
- **死信队列**：处理失败消息，避免无限重试
- **TTL设置**：10分钟消息过期时间
- **持久化**：确保消息不丢失
- **手动ACK**：精确控制消息确认

## 5. 监控与告警

### 5.1 日志监控
- 任务创建日志
- 消息发送日志
- 任务处理耗时
- 错误类型统计

### 5.2 死信队列监控
- 死信消息数量
- 失败原因分析
- 告警通知机制

### 5.3 性能指标
- 协程池利用率
- 消息队列长度
- 任务处理延迟
- 系统并发量

## 6. 使用示例

### 6.1 初始化RabbitMQ
```go
// 在main.go中初始化
if err := mq.InitMq(); err != nil {
    log.Fatalf("初始化RabbitMQ失败: %v", err)
}
```

### 6.2 启动后台服务
```go
// 启动任务处理服务
go service.OutPaintingBackgroundService()
```

### 6.3 创建任务
```go
// 创建任务并发送到队列
err := iTaskService.ProCreatePictureOutPaintingTask(&req, userId)
```

## 7. 故障排查

### 7.1 常见问题
1. **连接失败**：检查RabbitMQ服务状态和配置
2. **消息堆积**：调整协程池大小或优化处理逻辑
3. **死信过多**：分析失败原因，优化错误处理
4. **内存泄漏**：确保正确释放通道和确认消息

### 7.2 调试技巧
- 查看RabbitMQ管理界面
- 启用详细日志
- 监控死信队列
- 使用测试消息验证流程

## 8. 扩展建议

### 8.1 水平扩展
- 多个消费者实例
- 分布式任务处理
- 负载均衡

### 8.2 功能增强
- 优先级队列
- 延迟消息
- 批量处理
- 任务取消

这个架构通过RabbitMQ实现了任务解耦和削峰填谷，通过ants协程池实现了精确的并发控制，是一个高可用、高性能的异步任务处理方案。