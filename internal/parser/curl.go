package parser

import (
	"fmt"
	"regexp"
	"strings"

	"caseurl2md/internal/config"
)

// CurlParser cURL解析器
type CurlParser struct{}

// New 创建新的cURL解析器
func New() *CurlParser {
	return &CurlParser{}
}

// Parse 解析cURL命令
func (p *CurlParser) Parse(curlCmd string) (*config.RequestInfo, error) {
	info := &config.RequestInfo{
		Method:  "GET",
		Headers: make(map[string]string),
		Cookies: make(map[string]string),
	}

	if curlCmd == "" {
		return nil, fmt.Errorf("cURL命令为空")
	}

	// 清理和标准化cURL命令
	curlCmd = strings.TrimSpace(curlCmd)

	// 移除开头的curl关键字
	curlCmd = removeCurlKeyword(curlCmd)

	// 使用复杂解析器来正确处理所有参数
	complexInfo, err := parseComplexCurl(curlCmd)
	if err != nil {
		return nil, fmt.Errorf("解析cURL参数失败: %w", err)
	}

	// 复制复杂解析的结果
	info.URL = complexInfo.URL
	info.Method = complexInfo.Method
	info.Body = complexInfo.Body
	for k, v := range complexInfo.Headers {
		info.Headers[k] = v
	}

	if info.URL == "" {
		return nil, fmt.Errorf("未在cURL命令中找到URL")
	}

	// 如果有数据但方法仍然是GET，则设为POST
	if info.Body != "" && info.Method == "GET" {
		info.Method = "POST"
	}

	return info, nil
}

// removeCurlKeyword 移除curl关键字
func removeCurlKeyword(curlCmd string) string {
	// 处理可能带引号的curl命令
	curlCmd = strings.TrimPrefix(curlCmd, "curl")
	curlCmd = strings.TrimPrefix(curlCmd, "CURL")
	curlCmd = strings.TrimSpace(curlCmd)
	return curlCmd
}

// parseArguments 解析cURL参数 - 使用简单有效的方法
func parseArguments(args string, info *config.RequestInfo) error {
	// 1. 提取URL - 提取最后一个URL作为目标URL
	// 首先尝试提取带引号的URL，然后提取不带引号的
	quotedUrlRegex := regexp.MustCompile(`https?://[^\s"']+`)
	urlMatches := quotedUrlRegex.FindAllString(args, -1)
	if len(urlMatches) > 0 {
		// 取最后一个URL作为目标URL
		lastUrl := urlMatches[len(urlMatches)-1]
		// 清理URL末尾可能的引号
		lastUrl = strings.Trim(lastUrl, `"'`)
		info.URL = lastUrl
	}

	// 2. 专门处理 --data-binary 参数 - 使用更强大的方法处理复杂JSON
	info.Body = extractDataBinary(args)

	// 3. 默认方法
	if info.Body != "" && info.Method == "GET" {
		info.Method = "POST"
	}

	return nil
}

// parseHeaders 解析所有的 -H headers
func parseHeaders(args string, info *config.RequestInfo) {
	// 分割参数并逐个分析
	words := strings.Fields(args)

	for i := 0; i < len(words); i++ {
		word := words[i]
		if word == "-H" || word == "--header" {
			if i+1 < len(words) {
				headerValue := words[i+1]
				// 解析单个header
				if err := parseHeader(headerValue, info.Headers); err == nil {
					// 成功解析header
				}
				i++ // 跳过下一个词，因为它是header值
			}
		}
	}
}

// parseHeader 解析header
func parseHeader(header string, headers map[string]string) error {
	parts := strings.SplitN(header, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("无效的header格式: %s", header)
	}

	headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	return nil
}

// isURL 检查字符串是否像URL
func isURL(str string) bool {
	// 简单的URL检测
	return strings.HasPrefix(str, "http://") ||
		   strings.HasPrefix(str, "https://") ||
		   strings.Contains(str, "://")
}

// extractDataParameter 提取指定类型的data参数，处理复杂JSON
func extractDataParameter(args string, paramType string) string {
	// 查找参数的位置
	paramIndex := strings.Index(args, paramType)
	if paramIndex == -1 {
		return ""
	}

	// 跳过参数标识
	startIndex := paramIndex + len(paramType)

	// 跳过空白字符
	for startIndex < len(args) && (args[startIndex] == ' ' || args[startIndex] == '\t') {
		startIndex++
	}

	// 如果找到了引号，提取引号内的内容
	if startIndex >= len(args) {
		return ""
	}

	quote := args[startIndex]
	if quote == '"' || quote == '\'' {
		startIndex++ // 跳过开始的引号

		// 查找配对的结束引号，正确处理转义
		i := startIndex
		result := strings.Builder{}

		for i < len(args) {
			char := args[i]

			if char == '\\' && i+1 < len(args) {
				// 处理转义字符 - 保留转义的内容
				nextChar := args[i+1]
				if nextChar == '"' || nextChar == '\'' || nextChar == '\\' {
					result.WriteByte(nextChar) // 只添加被转义的字符，不添加反斜杠
					i += 2
					continue
				} else {
					// 其他转义字符，保留原始形式
					result.WriteByte(char)
					i++
					if i < len(args) {
						result.WriteByte(args[i])
						i++
					}
					continue
				}
			}

			if char == quote {
				// 找到结束引号
				return result.String()
			}

			result.WriteByte(char)
			i++
		}

		// 如果没有找到结束引号，返回已收集的内容
		return result.String()
	}

	// 改进：如果第一个字符不是引号，尝试智能提取JSON内容
	return extractUnquotedData(args, startIndex)
}

// extractDataBinary 提取--data-binary参数，处理复杂JSON（保留向后兼容）
func extractDataBinary(args string) string {
	return extractDataParameter(args, "--data-binary")
}

// extractUnquotedData 智能提取未加引号的数据内容
func extractUnquotedData(args string, startIndex int) string {
	if startIndex >= len(args) {
		return ""
	}

	i := startIndex
	result := strings.Builder{}
	bracketCount := 0
	braceCount := 0
	inString := false
	stringChar := byte(0)

	for i < len(args) {
		char := args[i]

		// 处理转义字符
		if char == '\\' && i+1 < len(args) {
			result.WriteByte(char)
			i++
			if i < len(args) {
				result.WriteByte(args[i])
				i++
			}
			continue
		}

		// 处理字符串内容
		if inString {
			result.WriteByte(char)
			if char == stringChar {
				inString = false
				stringChar = 0
			}
			i++
			continue
		}

		// 检测字符串开始
		if char == '"' || char == '\'' {
			inString = true
			stringChar = char
			result.WriteByte(char)
			i++
			continue
		}

		// 计算括号层级
		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
		} else if char == '[' {
			bracketCount++
		} else if char == ']' {
			bracketCount--
		}

		// 如果遇到参数分隔符且括号都已闭合，结束提取
		if (char == ' ' || char == '\t') && braceCount == 0 && bracketCount == 0 {
			break
		}

		// 如果遇到下一个参数的开始标志，结束提取
		if char == '-' && braceCount == 0 && bracketCount == 0 {
			// 检查是否是下一个参数（如 -H, -X, --data 等）
			if i+1 < len(args) && (args[i+1] == ' ' || args[i+1] == '-') {
				break
			}
		}

		result.WriteByte(char)
		i++
	}

	return result.String()
}

// 私有辅助函数，用于处理复杂的cURL解析场景
func parseComplexCurl(curlCmd string) (*config.RequestInfo, error) {
	// 使用正则表达式处理更复杂的情况
	re := regexp.MustCompile(`(?:-X|--request)\s+(['"]?)([A-Z]+)$1`)
	matches := re.FindStringSubmatch(curlCmd)

	info := &config.RequestInfo{
		Method:  "GET",
		Headers: make(map[string]string),
		Cookies: make(map[string]string),
	}

	if len(matches) > 2 {
		info.Method = matches[2]
	}

	// 解析headers - 使用更强的匹配来处理复杂header值，支持无引号和有引号的情况
	// 使用两种不同的正则表达式来处理带引号和不带引号的情况
	headerReQuoted := regexp.MustCompile(`(?:-H|--header)\s+["']([^"']+)["']`)
	headerReUnquoted := regexp.MustCompile(`(?:-H|--header)\s+([^"'\s][^\s]*?)\s`)

	var headerMatches [][]string
	headerMatches = append(headerMatches, headerReQuoted.FindAllStringSubmatch(curlCmd, -1)...)
	headerMatches = append(headerMatches, headerReUnquoted.FindAllStringSubmatch(curlCmd, -1)...)

	for _, match := range headerMatches {
		if len(match) > 1 {
			headerStr := match[1] // match[1]是header值
			// 解析单个header
			if err := parseHeader(headerStr, info.Headers); err == nil {
				// 成功解析header
			}
		}
	}

	// 解析cookies - 处理 -b 或 --cookie 参数
	parseCookies(curlCmd, info)

	// 解析所有类型的data参数，优先级：data-binary > data-raw > data > -d
	info.Body = extractDataParameter(curlCmd, "--data-binary")
	if info.Body == "" {
		info.Body = extractDataParameter(curlCmd, "--data-raw")
	}
	if info.Body == "" {
		info.Body = extractDataParameter(curlCmd, "--data")
	}
	if info.Body == "" {
		info.Body = extractDataParameter(curlCmd, "-d")
	}

	// 解析URL - 提取命令行中的第一个URL（curl命令的URL通常在最前面）
	// 使用更精确的正则表达式，匹配作为独立参数的URL，排除headers中的URL
	urlRe := regexp.MustCompile(`^\s*curl\s+['"]?(https?://[^'"\s]+)`)
	urlMatches := urlRe.FindStringSubmatch(curlCmd)
	if len(urlMatches) > 1 {
		info.URL = urlMatches[1]
	} else {
		// 如果前面的模式没匹配到，使用备用方案：查找第一个以http开头的URL
		backupUrlRe := regexp.MustCompile(`['"]?(https?://bytest\.bytedance\.net[^'"\s]+)['"]?`)
		backupMatches := backupUrlRe.FindStringSubmatch(curlCmd)
		if len(backupMatches) > 1 {
			info.URL = backupMatches[1]
		}
	}

	return info, nil
}