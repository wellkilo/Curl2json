package processor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"caseurl2md/internal/config"
	"caseurl2md/internal/extractor"
	"caseurl2md/internal/http"
	"caseurl2md/internal/parser"
	"caseurl2md/internal/validator"
)

// Processor 主处理器
type Processor struct {
	config    *config.Config
	curlParser *parser.CurlParser
	httpExecutor *http.Executor
	validator *validator.ResponseValidator
	treeExtractor *extractor.TreeExtractor
}

// New 创建新的处理器
func New(cfg *config.Config) *Processor {
	return &Processor{
		config:       cfg,
		curlParser:   parser.New(),
		httpExecutor: http.New(cfg.Timeout, cfg.Verbose),
		validator:    validator.New(cfg.Verbose),
		treeExtractor: extractor.New(cfg.TitleKeys, cfg.ChildrenKeys, cfg.Verbose),
	}
}

// Process 处理输入并返回结果
func (p *Processor) Process(input string, requestInfo *config.RequestInfo) ([]byte, error) {
	var req *config.RequestInfo
	var err error

	if input != "" {
		// 解析cURL命令
		req, err = p.curlParser.Parse(input)
		if err != nil {
			return nil, fmt.Errorf("cURL解析失败: %w", err)
		}
	} else if requestInfo != nil {
		// 使用提供的请求信息
		req = requestInfo
	} else {
		return nil, fmt.Errorf("没有提供输入")
	}

	// 执行HTTP请求
	responseData, err := p.httpExecutor.Execute(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求执行失败: %w", err)
	}

	// 校验响应
	if err := p.validator.Validate(responseData); err != nil {
		return nil, fmt.Errorf("响应校验失败: %w", err)
	}

	// 新增：检查是否为错误响应
	if p.isErrorResponse(responseData) {
		return nil, fmt.Errorf("服务器返回错误响应，无法提取业务数据")
	}

	// 抽取树状结构
	result, err := p.treeExtractor.Extract(responseData)
	if err != nil {
		// 保存原始响应用于调试
		if p.config.Verbose {
			debugFile := fmt.Sprintf("debug_response_%s.json", time.Now().Format("20060102_150405"))
			debugPath := filepath.Join(os.TempDir(), debugFile)
			if writeErr := os.WriteFile(debugPath, responseData, 0644); writeErr == nil {
				fmt.Printf("调试: 原始响应已保存到: %s\n", debugPath)
			}
		}
		return nil, fmt.Errorf("树状结构抽取失败: %w", err)
	}

	return result, nil
}

// GetAnalysis 获取输入分析（用于调试）
func (p *Processor) GetAnalysis(input string) (map[string]interface{}, error) {
	req, err := p.curlParser.Parse(input)
	if err != nil {
		return nil, err
	}

	analysis := make(map[string]interface{})
	analysis["parsed_url"] = req.URL
	analysis["parsed_method"] = req.Method
	analysis["parsed_headers"] = req.Headers
	analysis["has_body"] = req.Body != ""

	if len(req.Body) > 0 {
		analysis["body_length"] = len(req.Body)
		// 限制body内容显示长度
		if len(req.Body) > 100 {
			analysis["body_preview"] = req.Body[:100] + "..."
		} else {
			analysis["body_preview"] = req.Body
		}
	}

	return analysis, nil
}

// ValidateOnly 仅校验响应格式（用于测试）
func (p *Processor) ValidateOnly(responseData []byte) error {
	return p.validator.Validate(responseData)
}

// ExtractOnly 仅执行树抽取（用于测试）
func (p *Processor) ExtractOnly(responseData []byte) ([]byte, error) {
	return p.treeExtractor.Extract(responseData)
}

// ParseCurlOnly 仅解析cURL（用于测试）
func (p *Processor) ParseCurlOnly(curlCmd string) (*config.RequestInfo, error) {
	return p.curlParser.Parse(curlCmd)
}

// GetExtractor 获取树抽取器实例
func (p *Processor) GetExtractor() *extractor.TreeExtractor {
	return p.treeExtractor
}

// isErrorResponse 检查响应是否为错误响应
func (p *Processor) isErrorResponse(responseData []byte) bool {
	var response map[string]interface{}
	if err := json.Unmarshal(responseData, &response); err != nil {
		return true // 如果无法解析为JSON，认为是错误响应
	}

	// 检查是否包含错误相关的字段
	if errCode, exists := response["errCode"]; exists {
		if errCodeVal, ok := errCode.(float64); ok && errCodeVal != 0 {
			return true
		}
	}

	// 检查是否包含错误消息
	if message, exists := response["message"]; exists {
		if messageStr, ok := message.(string); ok &&
		   strings.Contains(strings.ToLower(messageStr), "error") ||
		   strings.Contains(strings.ToLower(messageStr), "auth") ||
		   strings.Contains(strings.ToLower(messageStr), "unauthorized") {
			return true
		}
	}

	// 检查是否缺少关键的TestCaseMind结构
	if data, exists := response["data"]; exists {
		if dataMap, ok := data.(map[string]interface{}); ok {
			if _, hasTestCaseMind := dataMap["TestCaseMind"]; !hasTestCaseMind {
				return true // 如果data中没有TestCaseMind字段，认为是错误响应
			}
		}
	} else {
		return true // 如果没有data字段，认为是错误响应
	}

	return false
}

// GuessStructure 尝试猜测JSON结构（用于调试）
func (p *Processor) GuessStructure(jsonData []byte) (map[string]interface{}, error) {
	return p.treeExtractor.GetStats(jsonData)
}