package service

import (
	"backend/internal/api/siliconflowapi/siliconflow"
	"backend/internal/consts"
	"backend/internal/ecode"
	"backend/internal/model/entity"
	"backend/internal/model/request/iTask"
	iTaskRes "backend/internal/model/response/iTask"
	"backend/internal/repository"
	"backend/pkg/mq"
	"errors"
	"fmt"
	"github.com/panjf2000/ants/v2"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ITaskService struct {
	ITaskRepo *repository.ITaskRepository
}

func NewITaskService() *ITaskService {
	return &ITaskService{
		ITaskRepo: repository.NewITaskRepository(),
	}
}

var aiGenPool *ants.Pool
var aiGenPoolOnce sync.Once

// 保证只初始化一次谢晨池
func GetAiGenPool() *ants.Pool {
	aiGenPoolOnce.Do(func() {
		var err error
		aiGenPool, err = ants.NewPool(4,
			ants.WithMaxBlockingTasks(20),
			ants.WithPreAlloc(true),
			ants.WithNonblocking(true))
		if err != nil {
			panic(fmt.Sprintf("创建协程池失败: %v", err))
		}
	})
	return aiGenPool
}

// 后台协程，负责处理AI图片拓展任务
// 后台协程，负责处理AI图片拓展任务
func OutPaintingBackgroundService() {
	log.Println("启动外绘背景服务...")
	aiPool := GetAiGenPool()
	ch := mq.GetChannel()
	defer mq.ReleaseChannel(ch)

	log.Printf("连接到消息队列: %s", consts.MQOutPaintingQueueName)
	msgs, err := ch.Consume(
		consts.MQOutPaintingQueueName,
		consts.OutPaintingConsumerName,
		false, // 手动ACK
		false, // 非排他
		false, // 非阻塞
		false, // 不等待
		nil,   // 额外参数
	)
	if err != nil {
		log.Panicf("注册消费者失败: %v", err)
	}
	log.Printf("成功注册消费者: %s", consts.OutPaintingConsumerName)

	type TaskErr struct {
		Id  uint64
		Err error
	}

	errChan := make(chan TaskErr, 24)
	go func() {
		log.Println("启动错误处理协程...")
		for chanerr := range errChan {
			log.Printf("[任务 %d] 开始处理错误: %v", chanerr.Id, chanerr.Err)

			taskSvc := NewITaskService()
			iTask, err := taskSvc.ITaskRepo.FindById(nil, chanerr.Id)
			if err != nil {
				log.Printf("[任务 %d] 获取任务失败: %v", chanerr.Id, err)
				continue
			}
			if iTask == nil {
				log.Printf("[任务 %d] 任务不存在，跳过处理", chanerr.Id)
				continue
			}

			log.Printf("[任务 %d] 更新任务状态为失败", chanerr.Id)
			updateMap := map[string]interface{}{
				"status":       consts.TaskStatusFailed,
				"exec_message": chanerr.Err.Error(),
			}

			err = taskSvc.ITaskRepo.UpdateByMap(nil, iTask.ID, updateMap)
			if err != nil {
				log.Printf("[任务 %d] 更新任务状态失败: %v", chanerr.Id, err)
			} else {
				log.Printf("[任务 %d] 任务状态已更新为失败", chanerr.Id)
			}
		}
	}()

	// 启动死信队列消费者
	go mq.StartDLXConsumer()

	go func() {
		log.Println("启动消息处理协程...")
		for d := range msgs {
			taskId, _ := strconv.ParseUint(string(d.Body), 10, 64)
			log.Printf("[任务 %d] 接收到新任务消息", taskId)

			taskProcessErr := aiPool.Submit(func() {
				log.Printf("[任务 %d] 任务开始处理", taskId)
				startTime := time.Now()

				taskSvc := NewITaskService()

				iTask, err := taskSvc.ITaskRepo.FindById(nil, taskId)
				if err != nil {
					log.Printf("[任务 %d] 获取任务失败: %v", taskId, err)
					d.Nack(false, true) // 可恢复错误，重新入队
					return
				}
				if iTask == nil {
					log.Printf("[任务 %d] 任务不存在，直接ACK", taskId)
					d.Ack(false)
					return
				}

				log.Printf("[任务 %d] 当前状态: %s", taskId, iTask.Status)
				if iTask.Status == consts.TaskStatusSucceed {
					log.Printf("[任务 %d] 任务已完成，直接ACK", taskId)
					d.Ack(false)
					return
				}

				if iTask.Status == consts.TaskStatusRunning {
					log.Printf("[任务 %d] 任务状态异常: 已在运行中", taskId)
					errChan <- TaskErr{
						Id:  iTask.ID,
						Err: errors.New("任务处理状态异常"),
					}
					d.Ack(false)
					return
				}

				log.Printf("[任务 %d] 更新任务状态为运行中", taskId)
				updateMap := map[string]interface{}{
					"status":       consts.TaskStatusRunning,
					"exec_message": "正在执行",
				}
				err = taskSvc.ITaskRepo.UpdateByMap(nil, iTask.ID, updateMap)
				if err != nil {
					log.Printf("[任务 %d] 更新任务状态失败: %v", taskId, err)
					errChan <- TaskErr{
						Id:  iTask.ID,
						Err: fmt.Errorf("更新任务状态失败: %w", err),
					}
					d.Ack(false)
					return
				}
				log.Printf("[任务 %d] 任务状态已更新为运行中", taskId)

				log.Printf("[任务 %d] 开始调用AI处理", taskId)
				result, err := taskSvc.processTask(iTask)
				if err != nil {
					// 判断错误类型，决定是重新入队还是进入死信队列
					if isRecoverableError(err) {
						log.Printf("[任务 %d] 可恢复错误，重新入队: %v", taskId, err)
						d.Nack(false, true) // 重新入队重试
					} else {
						log.Printf("[任务 %d] 不可恢复错误，进入死信队列: %v", taskId, err)
						d.Nack(false, false) // 进入死信队列
					}
					errChan <- TaskErr{
						Id:  iTask.ID,
						Err: fmt.Errorf("AI处理失败: %w", err),
					}
					return
				}
				log.Printf("[任务 %d] AI处理成功完成", taskId)

				log.Printf("[任务 %d] 更新任务状态为成功", taskId)
				updateMap = map[string]interface{}{
					"status":           consts.TaskStatusSucceed,
					"exec_message":     "执行成功",
					"expanded_pic_url": result.DirectURL,
					"ai_recap":         result.Analysis,
				}

				err = taskSvc.ITaskRepo.UpdateByMap(nil, iTask.ID, updateMap)
				if err != nil {
					log.Printf("[任务 %d] 保存任务结果失败: %v", taskId, err)
					errChan <- TaskErr{
						Id:  iTask.ID,
						Err: fmt.Errorf("保存任务结果失败: %w", err),
					}
					d.Ack(false)
					return
				}

				duration := time.Since(startTime)
				log.Printf("[任务 %d] 任务处理成功完成! 耗时: %v", taskId, duration)
				d.Ack(false)
			})

			if taskProcessErr != nil {
				log.Printf("[任务 %d] 任务提交到协程池失败: %v", taskId, taskProcessErr)
				d.Nack(false, true)
			} else {
				log.Printf("[任务 %d] 任务已成功提交到协程池", taskId)
			}
		}
	}()

	log.Println("外绘服务启动，死信监听已激活")
}

// 判断错误是否可恢复
func isRecoverableError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()

	// 可恢复错误：网络/超时/服务端错误
	switch {
	// 1. 网络层错误（TCP连接、DNS解析等）
	case strings.Contains(errMsg, "connection refused"),
		strings.Contains(errMsg, "connection reset"),
		strings.Contains(errMsg, "no such host"):
		return true

	// 2. 超时错误（客户端或服务端超时）
	case strings.Contains(errMsg, "timeout"),
		strings.Contains(errMsg, "timed out"):
		return true

	// 3. 服务端错误（5xx状态码）
	case strings.Contains(errMsg, "API返回错误: 5"):
		return true

	// 4. 速率限制（429状态码）
	case strings.Contains(errMsg, "API返回错误: 429"):
		return true

	// 5. 重试机制触发的临时错误
	case strings.Contains(errMsg, "API调用失败"):
		return true
	}

	// 不可恢复错误：客户端错误/数据问题
	switch {
	// 1. 请求数据错误（4xx状态码）
	case strings.Contains(errMsg, "API返回错误: 4"):
		return false

	// 2. JSON解析失败（数据格式错误）
	case strings.Contains(errMsg, "json.Marshal"),
		strings.Contains(errMsg, "响应解析失败"):
		return false

	// 3. 空结果（业务逻辑错误）
	case strings.Contains(errMsg, "AI返回结果为空"):
		return false
	}

	// 未知错误默认不可恢复（保守策略）
	return false
}

// 在service层处理任务
func (s *ITaskService) processTask(iTask *entity.ITask) (*siliconflow.OutPaintingResult, error) {
	// 调用AI API
	response, err := siliconflow.OutPaintingAPI(
		iTask.Prompt,
		iTask.OriginalPicUrl,
	)

	if err != nil {
		return nil, fmt.Errorf("AI处理失败: %w", err)
	}

	// 确保有返回结果
	if len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
		return nil, errors.New("AI返回结果为空")
	}

	// 解析特定格式的AI响应
	return siliconflow.ParseOutPaintingResult(response.Choices[0].Message.Content)
}

// 修复1: 实现原ProCreatePictureOutPaintingTask功能
func (s *ITaskService) ProCreatePictureOutPaintingTask(req *iTask.TaskRequest, userId uint64) *ecode.ErrorWithCode {
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

	// 发送MQ消息
	s.sendToMQ(&task)
	return nil
}

// 修复2: 实现GetITaskVOList功能
func (s *ITaskService) GetITaskVOList(userId uint64) ([]iTaskRes.ITaskVO, *ecode.ErrorWithCode) {
	tasks, err := s.ITaskRepo.FindByUserId(nil, userId)
	if err != nil {
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}

	vos := make([]iTaskRes.ITaskVO, 0, len(tasks))
	for _, task := range tasks {
		vos = append(vos, iTaskRes.ITaskVO{
			ID:             task.ID,
			Name:           task.Name,
			OriginalPicUrl: task.OriginalPicUrl,
			ExpandedPicUrl: task.ExpandedPicUrl,
			Status:         task.Status,
			CreateTime:     task.CreateTime,
		})
	}
	return vos, nil
}

// 修复5: 实现DeleteImageExpandTask功能
func (s *ITaskService) DeleteImageExpandTask(id uint64) *ecode.ErrorWithCode {
	err := s.ITaskRepo.Delete(nil, id)
	if err != nil {
		return ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "数据库错误")
	}
	return nil
}
func (s *ITaskService) sendToMQ(task *entity.ITask) {
	// 获取MQ连接池
	pool := mq.GetChannelPool()

	// 将任务ID转换为消息体
	message := []byte(strconv.FormatUint(task.ID, 10))

	// 发布消息到RabbitMQ
	if err := pool.PublishMessage(message); err != nil {
		log.Printf("MQ消息发送失败! 任务ID: %d, 错误: %v", task.ID, err)

		// 如果消息发送失败，回滚任务状态为"等待"
		updateMap := map[string]interface{}{
			"status":       consts.TaskStatusWait,
			"exec_message": "MQ发送失败，等待重试",
		}

		_ = s.ITaskRepo.UpdateByMap(nil, task.ID, updateMap)
	} else {
		log.Printf("MQ消息成功发送! 任务ID: %d", task.ID)
	}
}
