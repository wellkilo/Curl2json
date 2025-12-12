package validator

import (
	"encoding/json"
	"fmt"
	"strings"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ResponseValidator 响应校验器
type ResponseValidator struct {
	verbose bool
}

// New 创建新的响应校验器
func New(verbose bool) *ResponseValidator {
	return &ResponseValidator{
		verbose: verbose,
	}
}

// Validate 校验HTTP响应
func (v *ResponseValidator) Validate(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("响应体为空")
	}

	if v.verbose {
		fmt.Printf("开始校验响应，响应体大小: %d 字节\n", len(data))
		fmt.Printf("响应体前100字符: %s\n", string(data[:min(100, len(data))]))
	}

	// 尝试解析JSON
	var js json.RawMessage
	if err := json.Unmarshal(data, &js); err != nil {
		// 输出详细的JSON解析错误信息
		if v.verbose {
			fmt.Printf("JSON解析失败: %v\n", err)
			fmt.Printf("原始响应数据: %s\n", string(data[:min(500, len(data))]))
		}
		return fmt.Errorf("JSON解析失败: %w", err)
	}

	if v.verbose {
		fmt.Println("响应校验通过，格式为有效的JSON")
	}

	return nil
}

// IsJSONContentType 检查Content-Type是否为JSON
func (v *ResponseValidator) IsJSONContentType(contentType string) bool {
	if contentType == "" {
		return false
	}

	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "application/json") ||
		   strings.Contains(ct, "text/json") ||
		   strings.Contains(ct, "application/vnd.api+json")
}