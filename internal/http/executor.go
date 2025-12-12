package http

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"caseurl2md/internal/config"
)

// Executor HTTP请求执行器
type Executor struct {
	timeout time.Duration
	verbose bool
}

// New 创建新的HTTP执行器
func New(timeout time.Duration, verbose bool) *Executor {
	return &Executor{
		timeout: timeout,
		verbose: verbose,
	}
}

// Execute 执行HTTP请求
func (e *Executor) Execute(info *config.RequestInfo) ([]byte, error) {
	if e.verbose {
		fmt.Printf("执行HTTP请求: %s %s\n", info.Method, info.URL)
		fmt.Printf("=== DEBUG: Headers Count: %d ===\n", len(info.Headers))
		for key, value := range info.Headers {
			maskedValue := e.maskSensitiveHeader(key, value)
			fmt.Printf("Header: %s: %s\n", key, maskedValue)
			// 检查关键的API特定headers
			if key == "servicefunc" || key == "service" || key == "projectid" || key == "x-trigger-source" || key == "x-onesite-space-id" {
				fmt.Printf("  ⭐ 关键业务Header: %s = %s\n", key, maskedValue)
			}
		}
		if info.Body != "" {
			fmt.Printf("Body: %s\n", info.Body)
			fmt.Printf("Body Length: %d bytes\n", len(info.Body))
			// 检查JSON格式
			if strings.HasPrefix(info.Body, "{") {
				fmt.Printf("✅ Body format: Valid JSON start\n")
			} else {
				fmt.Printf("❌ Body format: May not be valid JSON\n")
			}
		}
	}

	// 创建请求体
	var body io.Reader
	if info.Body != "" {
		body = bytes.NewBufferString(info.Body)
	}

	// 创建HTTP请求
	req, err := http.NewRequest(info.Method, info.URL, body)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	for key, value := range info.Headers {
		req.Header.Set(key, value)
	}

	// 如果没有设置Content-Type但有请求体，设置为application/json
	if info.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: e.timeout,
	}

	if e.verbose {
		fmt.Println("开始发送请求...")
	}

	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求执行失败: %w", err)
	}
	defer resp.Body.Close()

	if e.verbose {
		fmt.Printf("收到响应，状态码: %d %s\n", resp.StatusCode, resp.Status)
	}

	// 读取响应体（无论状态码如何）
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 检查状态码但不立即返回错误，而是记录警告
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if e.verbose {
			fmt.Printf("警告: 服务器返回非2xx状态码: %d %s\n", resp.StatusCode, resp.Status)
			fmt.Printf("响应体长度: %d 字节\n", len(bodyBytes))
			if len(bodyBytes) > 0 {
				preview := string(bodyBytes)
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				fmt.Printf("响应体预览: %s\n", preview)
			}
		}
		// 不要直接返回错误，继续处理响应体
		// 调用者可以根据需要决定是否处理非2xx���应
	}

	if e.verbose {
		fmt.Printf("成功读取响应体，大小: %d 字节\n", len(bodyBytes))
	}

	return bodyBytes, nil
}

// maskSensitiveHeader 遮蔽敏感header信息
func (e *Executor) maskSensitiveHeader(key, value string) string {
	lowerKey := strings.ToLower(key)

	switch lowerKey {
	case "authorization", "cookie", "set-cookie", "x-api-key", "x-auth-token":
		if len(value) > 8 {
			return value[:4] + "***" + value[len(value)-4:]
		}
		return "***"
	default:
		return value
	}
}