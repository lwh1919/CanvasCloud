package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os" // Added for os.Getenv
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// UserRegisterRequest 注册请求结构体
type UserRegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func TestConcurrentRegistration(t *testing.T) {
	const (
		userCount   = 1000
		registerURL = "http://localhost:8001/api/v1/users/register"
		timeout     = 10 * time.Second // 延长超时时间
		concurrency = 100              // 控制并发数
	)

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxConnsPerHost: concurrency, // 控制并发
		},
	}

	sem := make(chan struct{}, concurrency) // 并发控制信号量
	var wg sync.WaitGroup
	var successCount, failCount int32

	wg.Add(userCount)

	for i := 0; i < userCount; i++ {
		sem <- struct{}{} // 获取信号量

		go func(index int) {
			defer func() {
				<-sem // 释放信号量
				wg.Done()
			}()

			reqBody := UserRegisterRequest{
				Username: fmt.Sprintf("test%04d", index+1),
				Email:    fmt.Sprintf("test%04d@example.com", index+1),
				Password: os.Getenv("TEST_PASSWORD"), // 从环境变量获取测试密码
			}

			jsonBody, _ := json.Marshal(reqBody)

			req, err := http.NewRequest("POST", registerURL, bytes.NewBuffer(jsonBody))
			if err != nil {
				t.Logf("用户%d 创建请求失败: %v", index, err)
				atomic.AddInt32(&failCount, 1)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				t.Logf("用户%d 发送请求失败: %v", index, err)
				atomic.AddInt32(&failCount, 1)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Logf("用户%d 注册失败，状态码: %d", index, resp.StatusCode)
				atomic.AddInt32(&failCount, 1)
				return
			}

			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	t.Logf("注册测试完成: 成功=%d, 失败=%d, 总请求=%d",
		successCount, failCount, userCount)

	if failCount > 0 {
		t.Fatalf("测试失败: %d次请求未通过", failCount)
	}
}
