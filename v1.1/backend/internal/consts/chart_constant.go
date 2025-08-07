package consts

const (
	MQExchangeName = "mrbi" // 交换机名称
	MQRoutingKey   = "mrbi" // 路由键名称

	// 外绘任务队列和消费者
	MQOutPaintingQueueName  = "out_painting_tasks"
	OutPaintingConsumerName = "outpainting_consumer"

	// 任务类型
	MQDeadLetterExchangeName = "dlx.exchange"    // 死信交换机名称
	MQDeadLetterQueueName    = "dlx.queue"       // 死信队列名称
	MQDeadLetterRoutingKey   = "dlx.routing.key" // 死信路由键
	TaskStatusWait           = "wait"
	TaskStatusRunning        = "running"
	TaskStatusSucceed        = "succeed"
	TaskStatusFailed         = "failed"
)
