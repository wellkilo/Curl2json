package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"Curl2json/internal/config"
	"Curl2json/internal/processor"
)

var (
	curlFile      string
	fromCurl      string
	rawCurl       string
	url           string
	method        string
	headers       []string
	data          string
	cookies       string
	out           string
	titleKeys     []string
	childrenKeys  []string
	timeout       int
	verbose       bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "Curl2json",
	Short: "cURL请求到树状JSON转换工具",
	Long: `将cURL命令转换为精简的树状JSON结构工具。

该工具能够：
1. 解析cURL命令
2. 执行HTTP请求
3. 从JSON响应中抽取树状结构
4. 输出仅包含case_title和children的精简JSON

支持三种输入方式：
- 从stdin读取cURL命令
- 从文件读取cURL命令
- 通过命令行参数直接指定请求信息`,
	Example: `  # 直接使用cURL命令
  ./Curl2json --from-curl 'curl "http://example.com/api" -H "Authorization: Bearer token"'

  # 从文件读取cURL
  ./Curl2json --curl-file curl.txt --out result.json

  # 手动指定参数
  ./Curl2json --url "http://api.example.com/data" --header "Content-Type: application/json" --method POST`,
	RunE: runRoot,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// 输入相关flags
	rootCmd.Flags().StringVar(&fromCurl, "from-curl", "", "直接从命令行接收cURL命令")
	rootCmd.Flags().StringVar(&rawCurl, "raw-curl", "", "接收完整的cURL命令字符串（支��多行格式）")
	rootCmd.Flags().StringVar(&curlFile, "curl-file", "", "从文件读取cURL命令")
	rootCmd.Flags().StringVar(&url, "url", "", "请求URL（不使用cURL时必需）")
	rootCmd.Flags().StringVar(&method, "method", "GET", "请求方法")
	rootCmd.Flags().StringSliceVar(&headers, "header", []string{}, "请求头，格式为'Key: Value'，可多次使用")
	rootCmd.Flags().StringVar(&data, "data", "", "请求体数据")
	rootCmd.Flags().StringVar(&cookies, "cookies", "", "cookies字符串，格式为'key1=value1; key2=value2'")

	// 输出相关flags
	rootCmd.Flags().StringVar(&out, "out", "", "输出文件路径（默认为output_{timestamp}.json）")

	// 抽取规则相关flags
	rootCmd.Flags().StringSliceVar(&titleKeys, "title-key", []string{"case_title", "title", "name", "label"}, "节点内容字段候选键名，按优先级排序")
	rootCmd.Flags().StringSliceVar(&childrenKeys, "children-keys", []string{"children", "nodes", "sub_cases", "items", "data"}, "子节点数组候选键名，按优先级排序")

	// 其他flags
	rootCmd.Flags().IntVar(&timeout, "timeout", 30, "HTTP请求超时时间（秒）")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示详细日志")

	// 重要：禁用 Cobra 的默认解析行为，防止它错误解析 cURL 命令中的参数
	rootCmd.DisableFlagParsing = false
}

func runRoot(cmd *cobra.Command, args []string) error {
	// 特殊处理：如果使用 --from-curl 参数，但存在额外参数，将它们合并到 fromCurl 中
	if fromCurl != "" && len(args) > 0 {
		// 将额外的参数追加到 fromCurl 命令中
		fromCurl = fromCurl + " " + strings.Join(args, " ")
	}

	// 验证输入���数
	if err := validateInput(); err != nil {
		return err
	}

	// 构建配置
	cfg := &config.Config{
		Timeout:      time.Duration(timeout) * time.Second,
		TitleKeys:    titleKeys,
		ChildrenKeys: childrenKeys,
		Verbose:      verbose,
	}

	// 获取输入源
	var input string
	var err error

	switch {
	case rawCurl != "":
		input = rawCurl
		if verbose {
			fmt.Println("使用 --raw-curl 参数接收完整cURL命令")
			fmt.Printf("完整cURL命令: %s\n", input)
		}
	case fromCurl != "":
		input = fromCurl
		if verbose {
			fmt.Println("从命令行参数读取cURL命令")
			fmt.Printf("完整cURL命令: %s\n", input)
		}
	case curlFile != "":
		input, err = readFromFile(curlFile)
		if err != nil {
			return fmt.Errorf("读取cURL文件失败: %w", err)
		}
		if verbose {
			fmt.Printf("从文件读取cURL命令: %s\n", curlFile)
		}
	case url != "":
		// 直接使用参数模式，不需要cURL
		input = ""
		if verbose {
			fmt.Printf("使用参数模式: %s %s\n", method, url)
		}
	default:
		// 从stdin读取
		input, err = readFromStdin()
		if err != nil {
			return fmt.Errorf("从stdin读取失败: %w", err)
		}
		if verbose {
			fmt.Println("从stdin读取cURL命令")
		}
	}

	// 设置默认输出文件
	if out == "" {
		timestamp := time.Now().Format("20060102_150405")
		out = fmt.Sprintf("output_%s.json", timestamp)
	}

	// 创建处理器并执行
	processor := processor.New(cfg)

	result, err := processor.Process(input, &config.RequestInfo{
		URL:     url,
		Method:  method,
		Headers: parseHeaders(headers),
		Cookies: parseCookies(cookies),
		Body:    data,
	})

	if err != nil {
		return err
	}

	// 写入输出文件
	if err := writeOutput(out, result); err != nil {
		return err
	}

	fmt.Printf("成功将结果写入文件: %s\n", out)
	return nil
}

func validateInput() error {
	// 检查是否有输入
	inputCount := 0
	if rawCurl != "" {
		inputCount++
	}
	if fromCurl != "" {
		inputCount++
	}
	if curlFile != "" {
		inputCount++
	}
	if url != "" {
		inputCount++
	}

	if inputCount == 0 {
		return fmt.Errorf("必须指定一种输入方式：--raw-curl, --from-curl, --curl-file, --url, 或者从stdin提供cURL命令")
	}

	if inputCount > 1 {
		return fmt.Errorf("只能指定一种输入方式")
	}

	return nil
}

func readFromFile(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func readFromStdin() (string, error) {
	var content []byte
	buf := make([]byte, 1024)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			content = append(content, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(string(content)), nil
}

func parseHeaders(headerSlice []string) map[string]string {
	headers := make(map[string]string)
	for _, h := range headerSlice {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return headers
}

func parseCookies(cookieStr string) map[string]string {
	cookies := make(map[string]string)
	if cookieStr == "" {
		return cookies
	}

	// 分割cookie字符串
	pairs := strings.Split(cookieStr, ";")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// 分割键值对
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" {
				cookies[key] = value
			}
		}
	}
	return cookies
}

func writeOutput(filename string, content []byte) error {
	return os.WriteFile(filename, content, 0644)
}