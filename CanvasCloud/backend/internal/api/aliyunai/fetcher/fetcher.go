package fetcher

import (
	"encoding/json"                        // JSON编解码
	"fmt"                                  // 格式化输出
	"log"                                  // 日志记录
	"resty.dev/v3"                         // HTTP客户端库
	"web_app2/config"                      // 项目配置模块，用于加载API密钥等配置
	"web_app2/internal/api/aliyunai/model" // 阿里云AI模型定义
	"web_app2/internal/ecode"              // 错误码处理模块
)

// 包级常量定义API端点
const (
	// 创建扩图任务的API地址
	CreateOutPaintingTaskURL = "https://dashscope.aliyuncs.com/api/v1/services/aigc/image2image/out-painting"

	// 获取任务结果的API地址（需要拼接task_id）
	GetOutPaintingTaskURL = "https://dashscope.aliyuncs.com/api/v1/tasks/%s"
)

// 创建扩图任务函数
// 参数: req - 包含扩图请求参数的结构体指针
// 返回值: 任务创建响应结构体指针 + 错误信息
func CreateOutPaintingTask(req *model.CreateOutPaintingTaskRequest) (*model.CreateOutPaintingTaskResponse, *ecode.ErrorWithCode) {
	// 1. 参数校验 - 确保请求不为空
	if req == nil {
		// 返回参数错误: PARAMS_ERROR (40001)
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "请求参数不能为空")
	}

	// 2. 创建HTTP客户端
	client := resty.New()

	// 3. 将请求结构体序列化为JSON字节数组
	body, err := json.Marshal(req)
	if err != nil {
		// 返回系统错误: SYSTEM_ERROR (50001) 序列化失败
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "请求参数序列化失败")
	}

	// 4. 加载配置获取API密钥
	cfg := config.LoadConfig()

	// 5. 发送POST请求到阿里云API
	resp, err := client.R(). // 创建请求对象
					SetHeader("Authorization", fmt.Sprintf("Bearer %s", cfg.AliYunAi.ApiKey)). // 设置认证头
					SetHeader("X-DashScope-Async", "enable").                                  // 启用异步模式
					SetHeader("Content-Type", "application/json").                             // 设置JSON内容类型
					SetBody(body).                                                             // 设置请求体
					Post(CreateOutPaintingTaskURL)                                             // 指定POST方法和URL

	// 6. 处理HTTP请求错误
	if err != nil {
		log.Println("请求失败:", err) // 记录错误日志
		// 返回操作错误: OPERATION_ERROR (50002)
		return nil, ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "AI扩图失败")
	}

	// 7. 检查HTTP状态码 (非200均为错误)
	if resp.StatusCode() != 200 {
		// 返回操作错误: OPERATION_ERROR (50002)
		return nil, ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "AI扩图失败")
	}

	// 8. 解析JSON响应到结构体
	var result model.CreateOutPaintingTaskResponse
	if err := json.Unmarshal(resp.Bytes(), &result); err != nil {
		// 返回系统错误: SYSTEM_ERROR (50001) 解析失败
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "响应解析失败")
	}

	// 9. 检查阿里云API返回的错误码
	if result.Code != "" {
		// 返回系统错误并附加阿里云错误信息
		return nil, ecode.GetErrWithDetail(
			ecode.SYSTEM_ERROR,
			"请求异常，AI扩图失败："+result.Message)
	}

	// 10. 返回成功响应
	return &result, nil
}

// 获取扩图任务状态函数
// 参数: taskId - 任务ID字符串
// 返回值: 任务状态响应结构体指针 + 错误信息
func GetOutPaintingTaskResponse(taskId string) (*model.GetOutPaintingResponse, *ecode.ErrorWithCode) {
	// 1. 参数校验 - 确保任务ID不为空
	if taskId == "" {
		// 返回参数错误: PARAMS_ERROR (40001)
		return nil, ecode.GetErrWithDetail(ecode.PARAMS_ERROR, "任务ID不能为空")
	}

	// 2. 创建HTTP客户端
	client := resty.New()

	// 3. 发送GET请求到阿里云任务查询API
	resp, err := client.R(). // 创建请求对象
					SetHeader("Authorization", fmt.Sprintf("Bearer %s", config.LoadConfig().AliYunAi.ApiKey)). // 设置认证头
					Get(fmt.Sprintf(GetOutPaintingTaskURL, taskId))                                            // 格式化URL并发送GET请求

	// 4. 处理HTTP请求错误
	if err != nil {
		log.Println("请求失败:", err) // 记录错误日志
		// 返回操作错误: OPERATION_ERROR (50002)
		return nil, ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "获取任务状态失败")
	}

	// 5. 检查HTTP状态码 (非200均为错误)
	if resp.StatusCode() != 200 {
		// 返回操作错误: OPERATION_ERROR (50002)
		return nil, ecode.GetErrWithDetail(ecode.OPERATION_ERROR, "获取任务状态失败")
	}

	// 6. 解析JSON响应到结构体
	var result model.GetOutPaintingResponse
	if err := json.Unmarshal(resp.Bytes(), &result); err != nil {
		// 返回系统错误: SYSTEM_ERROR (50001) 解析失败
		return nil, ecode.GetErrWithDetail(ecode.SYSTEM_ERROR, "响应解析失败")
	}

	// 7. 返回成功响应
	return &result, nil
}
