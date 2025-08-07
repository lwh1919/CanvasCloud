package siliconflow

import (
	"backend/config"
	"backend/internal/api/siliconflowapi/openai"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 新的图片拓展请求结构
func NewImageOutPaintingRequest(requirement string, imageURL string) *openai.LLMRequest {
	sysPrompt := fmt.Sprintf("你是一个专业的图像处理助手\n" +
		"请根据用户提供的图片拓展需求和原始图片URL，执行图片拓展操作\n" +
		"请求格式如下:\n" +
		"图片拓展需求:\n" +
		"{需求描述}\n" +
		"原始图片:\n" +
		"{图片URL}\n" +
		"响应格式要求如下:\n" +
		"处理结果URL:{生成图片URL}\n" +
		"图片分析:{图片分析总结}\n")

	return &openai.LLMRequest{
		Model: "baidu/ERNIE-4.5-300B-A47B",
		Messages: []openai.Message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: fmt.Sprintf("图片拓展需求:\n%s\n原始图片:\n%s", requirement, imageURL)},
		},
		Stream:    false, // 非流式响应模式
		MaxTokens: 8000,
	}
}

// OutPaintingAPI 图片拓展API入口
func OutPaintingAPI(requirement, imageURL string) (*openai.LLMResponse, error) {
	// 使用新的请求格式
	req := NewImageOutPaintingRequest(requirement, imageURL)

	apiKey := config.LoadConfig().SiliconflowConfig.APIkey
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal() failed: %v", err)
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	url := "https://api.siliconflow.cn/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	// 重试机制
	var resp *http.Response
	for i := 0; i < 3; i++ {
		resp, err = client.Do(httpReq)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("API调用失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API返回错误: %d - %s", resp.StatusCode, string(body))
	}

	// 解析标准响应
	var llmResponse openai.LLMResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResponse); err != nil {
		return nil, fmt.Errorf("响应解析失败: %v", err)
	}

	return &llmResponse, nil
}

// 解析图片拓展结果
type OutPaintingResult struct {
	DirectURL string // 直接可用的图片URL
	Analysis  string // AI分析总结
}

func ParseOutPaintingResult(content string) (*OutPaintingResult, error) {
	result := &OutPaintingResult{}

	// 查找关键信息
	if urlPrefix := "处理结果URL:"; strings.Contains(content, urlPrefix) {
		parts := strings.Split(content, urlPrefix)
		if len(parts) > 1 {
			if newline := strings.Index(parts[1], "\n"); newline != -1 {
				result.DirectURL = strings.TrimSpace(parts[1][:newline])
			} else {
				result.DirectURL = strings.TrimSpace(parts[1])
			}
		}
	}

	if analysisPrefix := "图片分析:"; strings.Contains(content, analysisPrefix) {
		parts := strings.Split(content, analysisPrefix)
		if len(parts) > 1 {
			result.Analysis = strings.TrimSpace(parts[1])
		}
	}

	// 确保至少有一个有效结果
	if result.DirectURL == "" && result.Analysis == "" {
		return nil, errors.New("无法解析AI返回的结果")
	}

	return result, nil
}
