package mq

import (
	"backend/config"
	"backend/internal/consts"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go" // RabbitMQ 官方库
)

// 全局连接池实例
var connPool *ChannelPool

// ChannelPool 定义通道连接池结构
type ChannelPool struct {
	conn *amqp.Connection   // RabbitMQ 服务器连接
	pool chan *amqp.Channel // 缓冲通道，用于存储和管理可用通道
}

// init 包初始化函数 - 在程序启动时自动执行
func InitMq() error {
	// 1. 初始化连接池配置
	cfg := config.LoadConfig().RabbitMQConfig // 从配置文件加载 RabbitMQ 配置

	// 2. 创建 RabbitMQ 连接
	// 格式: amqp://用户名:密码@主机:端口/
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/",
		cfg.UserName, cfg.Password,
		cfg.Host, cfg.Port))
	if err != nil {
		return err
		// 连接失败则终止程序
	}

	// 3. 初始化连接池
	connPool = &ChannelPool{
		conn: conn,
		pool: make(chan *amqp.Channel, 6), // 创建容量为5的缓冲通道
	}

	// 4. 创建临时通道用于设置 RabbitMQ 基础设施
	ch, err := conn.Channel()
	if err != nil {
		return err
		// 连接失败则终止程序
	}
	// 2. 声明死信交换机（新增）
	err = ch.ExchangeDeclare(
		consts.MQDeadLetterExchangeName,
		"direct",
		true, false, false, false, nil)
	failOnError(err, "Failed to declare DLX")

	// 3. 声明死信队列（新增）
	_, err = ch.QueueDeclare(
		consts.MQDeadLetterQueueName,
		true,  // 持久化
		false, // 非自动删除
		false, // 非排他
		false, // 不等待
		nil)
	failOnError(err, "Failed to declare DLQ")

	// 4. 绑定死信队列到死信交换机（新增）
	err = ch.QueueBind(
		consts.MQDeadLetterQueueName,
		consts.MQDeadLetterRoutingKey,
		consts.MQDeadLetterExchangeName,
		false, // 不等待
		nil)
	failOnError(err, "Failed to bind DLQ")

	// 5. 主队列声明（添加死信参数）
	args := amqp.Table{
		"x-dead-letter-exchange":    consts.MQDeadLetterExchangeName, // 死信目标交换机
		"x-dead-letter-routing-key": consts.MQDeadLetterRoutingKey,   // 死信路由键
		"x-message-ttl":             600000,                          // 消息10分钟后过期进入DLQ
	}
	// 5. 声明交换机 (Exchange)
	// 交换机功能类似邮局，负责将消息路由到一个或多个队列
	err = ch.ExchangeDeclare(
		consts.MQExchangeName, // 交换机名称
		"direct",              // 交换机类型: direct(直接匹配路由键)
		true,                  // 持久化 - 重启RabbitMQ后保留交换机
		false,                 // auto-delete: 当所有队列解绑后自动删除
		false,                 // internal: 仅供内部使用（非客户端使用）
		false,                 // no-wait: 不等待服务器确认
		args,                  // 额外参数
	)
	failOnError(err, "Failed to declare an exchange")

	// 6. 声明消息队列 (Queue)
	// 队列是实际存储消息的容器
	_, err = ch.QueueDeclare(
		consts.MQOutPaintingQueueName, // 队列名称
		true,                          // 持久化 - 重启RabbitMQ后保留队列
		false,                         // auto-delete: 所有消费者断开后自动删除
		false,                         // exclusive: 排他队列（仅当前连接可用）
		false,                         // no-wait: 不等待服务器确认
		nil,                           // 额外参数
	)
	if err != nil {
		return err
		// 连接失败则终止程序
	}

	// 7. 将队列绑定到交换机
	// 指定路由键(routing key)，匹配的消息将被路由到该队列
	err = ch.QueueBind(
		consts.MQOutPaintingQueueName, // 队列名称
		consts.MQRoutingKey,           // 路由键 - 消息发送时指定的匹配键
		consts.MQExchangeName,         // 交换机名称
		false,                         // no-wait: 不等待服务器确认
		nil,                           // 额外参数
	)
	if err != nil {
		return err
		// 连接失败则终止程序
	}

	// 8. 预创建通道并放入连接池
	for i := 0; i < cap(connPool.pool); i++ {
		channel, err := conn.Channel() // 创建新通道
		if err != nil {
			return err
			// 连接失败则终止程序
		}

		// 设置服务质量(QoS) - 控制每个消费者能处理的最大未确认消息数
		// 避免某个消费者堆积过多消息
		err = channel.Qos(
			20,    // 预取计数(prefetch count) - 每次最多接收20条未确认消息
			0,     // 预取大小 - 0表示不限制消息大小
			false, // global: 应用范围(false:仅当前消费者 true:所有消费者)
		)
		if err != nil {
			return err
			// 连接失败则终止程序
		}

		// 将通道放入池中
		connPool.pool <- channel
	}
	return nil
}

// 生产者区域：
// 🐇 ➤➤📩➤➤ [🔄交换机]
// |
// | (根据路由键)
// ▼
//
// 消息队列区域：
// [📦📦📦 队 列 仓 库 📦📦📦]
// │││     ┌──────┘↑└──────┐
// │││    🚪      🚪      🚪
// 通道接口区域：
// [▢ 通道1]  [▢ 通道2]  [▢ 通道3]
// │          │          │
// ▼          ▼          ▼
//
// 消费者区域：
// [👷‍♀️消费者1] [👷‍♂️消费者2] [🤖消费者3]
// ✋          ✋          ✋
// ⬇️ACK确认   ⬇️ACK确认   ⬇️ACK确认
// failOnError 错误处理辅助函数
func failOnError(err error, msg string) {
	if err != nil {
		// 格式化错误信息并终止程序
		log.Panicf("%s: %s", msg, err)
	}
}

// GetChannelPool 获取全局连接池实例
func GetChannelPool() *ChannelPool {
	return connPool
}

// GetChannel 从池中获取一个可用通道
// 注意: 使用后必须调用 ReleaseChannel 归还通道，否则会导致资源泄漏
func GetChannel() *amqp.Channel {
	// 从缓冲通道获取通道(如果池中没有可用通道，会阻塞等待)
	return <-connPool.pool
}

// ReleaseChannel 释放通道回到连接池
func ReleaseChannel(ch *amqp.Channel) {
	// 将通道放回缓冲通道
	connPool.pool <- ch
}

// PublishMessage 向 RabbitMQ 发布消息
func (connPool *ChannelPool) PublishMessage(message []byte) error {
	// 从池中获取通道
	ch := <-connPool.pool
	// 确保无论发生什么都将通道放回池中
	defer func() {
		connPool.pool <- ch
	}()

	// 使用通道发布消息到交换机
	err := ch.Publish(
		consts.MQExchangeName, // 交换机名称
		consts.MQRoutingKey,   // 路由键 - 与队列绑定键匹配
		false,                 // mandatory: 如果为true，找不到路由时返回错误
		false,                 // immediate: RabbitMQ已弃用(设置为false)
		amqp.Publishing{ // 消息属性
			ContentType: "text/plain", // 内容类型
			Body:        message,      // 消息体(byte切片)
		},
	)
	return err // 返回可能的错误
}
func StartDLXConsumer() {
	ch := GetChannel()
	defer ReleaseChannel(ch)

	msgs, err := ch.Consume(
		consts.MQDeadLetterQueueName,
		"dlx_consumer",
		false, // 手动ACK
		false,
		false,
		false,
		nil)
	failOnError(err, "Failed to start DLX consumer")

	go func() {
		for d := range msgs {
			log.Printf("死信告警: 任务 %s 进入DLQ, 原始路由键=%s, 错误原因=%v",
				d.Body,
				d.RoutingKey,
				d.Headers["x-death"]) // RabbitMQ自动添加的死亡记录

			// 实际项目中此处添加告警通知（邮件/钉钉等）
			// alertService.Notify("死信告警", string(d.Body))

			d.Ack(false) // 确认消费
		}
	}()
	log.Println("死信队列监听器已启动")
}
