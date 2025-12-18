package extractor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

// TreeExtractor 树抽取器
type TreeExtractor struct {
	titleKeys    []string
	childrenKeys []string
	verbose      bool
	maxDepth     int
}

// SimplifiedNode 简化的树节点结构
type SimplifiedNode struct {
	Name     string            `json:"name"`
	Children []*SimplifiedNode `json:"children"`
}

// New 创建新的树抽取器
func New(titleKeys, childrenKeys []string, verbose bool) *TreeExtractor {
	if len(titleKeys) == 0 {
		titleKeys = []string{"case_title", "title", "name", "label"}
	}
	if len(childrenKeys) == 0 {
		childrenKeys = []string{"children", "nodes", "sub_cases", "items", "data"}
	}

	return &TreeExtractor{
		titleKeys:    titleKeys,
		childrenKeys: childrenKeys,
		verbose:      verbose,
		maxDepth:     100, // 防止无限递归
	}
}

// Extract 从原始JSON中抽取树状结构
func (e *TreeExtractor) Extract(data []byte) ([]byte, error) {
	var rawData interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	if e.verbose {
		fmt.Printf("开始抽取树状结构，标题候选键: %v, 子节点候选键: %v\n", e.titleKeys, e.childrenKeys)
	}

	var result interface{}

	// 强制使用业务文本提取，避免技术元数据干扰
	if e.verbose {
		fmt.Println("强制使用业务文本提取模式...")
	}
	result = e.createDefaultStructure(rawData)
	if result == nil {
		return nil, fmt.Errorf("未找到有效的树状结构")
	}

	// 序列化结果，使用自定义函数避免Unicode转义
	output, err := marshalJSONWithoutEscape(result)
	if err != nil {
		return nil, fmt.Errorf("结果序列化失败: %w", err)
	}

	if e.verbose {
		fmt.Println("树状结构抽取完成")
	}

	return output, nil
}

// ExtractTextContent 从复杂的JSON数据中提取所有文本内容
func (e *TreeExtractor) ExtractTextContent(data interface{}) []string {
	var texts []string

	switch v := data.(type) {
	case string:
		if v != "" && e.isBusinessText(v) {
			texts = append(texts, v)
		}
	case map[string]interface{}:
		// 优先查找richText数组中的text字段
		if richTextArray, exists := v["richText"]; exists {
			if richTextItems, ok := richTextArray.([]interface{}); ok {
				for _, item := range richTextItems {
					if richTextObj, ok := item.(map[string]interface{}); ok {
						if textVal, textExists := richTextObj["text"]; textExists {
							if textStr, ok := textVal.(string); ok && textStr != "" && e.isBusinessText(textStr) {
								texts = append(texts, textStr)
							}
						}
					}
				}
			}
		}

		// 查找其他可能的text字段
		for key, value := range v {
			// 只关注包含text的字段
			if key == "text" || strings.Contains(key, "text") {
				if textVal, ok := value.(string); ok && textVal != "" && e.isBusinessText(textVal) {
					texts = append(texts, textVal)
				}
			} else if key == "title" || key == "name" || key == "label" || key == "message" || key == "description" {
				if textVal, ok := value.(string); ok && textVal != "" && e.isBusinessText(textVal) {
					texts = append(texts, textVal)
				}
			} else {
				// 递归处理嵌套结构
				texts = append(texts, e.ExtractTextContent(value)...)
			}
		}
	case []interface{}:
		for _, item := range v {
			texts = append(texts, e.ExtractTextContent(item)...)
		}
	default:
		// 对于其他类型，不处理，避免技术字段混入
	}

	return texts
}

// isBusinessText 简化的业务文本判断，过滤明显的技术字段
func (e *TreeExtractor) isBusinessText(text string) bool {
	if text == "" {
		return false
	}

	// 过滤明显的技术字段
	technicalKeywords := []string{
		"CreatedAt", "UpdatedAt", "TestCaseId", "ProductId", "errCode",
		"Status", "StatusCode", "session_id", "expiry_time", "token_type",
		"debug_info", "suggestions", "ERROR", "failed", "expired",
	}

	// 检查是否包含技术关键词
	for _, keyword := range technicalKeywords {
		if strings.Contains(text, keyword) {
			return false
		}
	}

	// 过滤技术数据格式
	if strings.HasPrefix(text, "[]") || strings.HasPrefix(text, "{}") ||
	   strings.HasPrefix(text, "map[") || strings.HasPrefix(text, "e+") {
		return false
	}

	// 过滤短英文技术词汇
	shortTechnicalWords := []string{"api", "url", "http", "get", "post"}
	for _, word := range shortTechnicalWords {
		if strings.EqualFold(text, word) {
			return false
		}
	}

	return true
}

// hasChinese 检查文本是否包含中文字符
func hasChinese(text string) bool {
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

// isEnglishBusinessText 检查是否为英文业务文本
func isEnglishBusinessText(text string) bool {
	// 过滤掉太短的文本
	if len(text) < 3 {
		return false
	}

	// 检查是否为有意义的英文词汇
	meaningfulWords := []string{
		"test", "check", "verify", "validate", "confirm", "review",
		"optimize", "performance", "scenario", "case", "step",
		"benchmark", "baseline", "regression", "functional",
		"integration", "monitor", "alert", "security", "login",
		"logout", "auth", "user", "admin", "system", "feature",
		"module", "component", "service", "api", "endpoint",
		"request", "response", "client", "server", "database",
	"frontend", "backend", "interface", "config", "setting",
	}

	textLower := strings.ToLower(text)
	for _, word := range meaningfulWords {
		if strings.Contains(textLower, word) {
			return true
		}
	}

	return false
}

// createDefaultStructure 基于��知结构直接解析TestCaseMind
func (e *TreeExtractor) createDefaultStructure(data interface{}) interface{} {
	if e.verbose {
		fmt.Println("开始解析TestCaseMind结构...")
	}

	// 直接解析TestCaseMind结构（基于已知的API响应格式）
	if testCaseMindNodes := e.parseTestCaseMindStructureDirect(data); testCaseMindNodes != nil {
		if e.verbose {
			fmt.Println("成功解析TestCaseMind结构")
		}
		return testCaseMindNodes
	}

	// 如果没有TestCaseMind结构，返回空结果
	if e.verbose {
		fmt.Println("未找到TestCaseMind结构，返回空结果")
	}
	return nil
}

// tryStandardTreeStructure 尝试解析标准树结构
func (e *TreeExtractor) tryStandardTreeStructure(data interface{}) interface{} {
	// 将数据转换为map以便访问
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		// 尝试处理数组
		if dataArray, ok := data.([]interface{}); ok {
			var roots []*SimplifiedNode
			for _, item := range dataArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if node := e.extractTree(itemMap, 0); node != nil {
						roots = append(roots, node)
					}
				}
			}
			if len(roots) > 0 {
				return roots
			}
		}
		return nil
	}

	// 查找是否有标准的树结构标识
	if node := e.extractTree(dataMap, 0); node != nil {
		return node
	}

	return nil
}

// parseTestCaseMindStructureDirect 直接解析TestCaseMind结构
func (e *TreeExtractor) parseTestCaseMindStructureDirect(data interface{}) interface{} {
	if e.verbose {
		fmt.Println("=== parseTestCaseMindStructureDirect 开始 ===")
	}

	// 将数据转换为map以便访问
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		if e.verbose {
			fmt.Printf("数据类型断言失败，期望map[string]interface{}，实际: %T\n", data)
		}
		return nil
	}

	// 查找data字段
	dataField, exists := dataMap["data"]
	if !exists {
		if e.verbose {
			fmt.Println("未找到data字段")
		}
		return nil
	}

	dataMap2, ok := dataField.(map[string]interface{})
	if !ok {
		if e.verbose {
			fmt.Printf("data字段类型断言失败，期望map[string]interface{}，实际: %T\n", dataField)
		}
		return nil
	}

	// 查找TestCaseMind字段
	testCaseMind, exists := dataMap2["TestCaseMind"]
	if !exists {
		if e.verbose {
			fmt.Println("未找到TestCaseMind字段")
		}
		return nil
	}

	testCaseMindStr, ok := testCaseMind.(string)
	if !ok {
		if e.verbose {
			fmt.Printf("TestCaseMind字段类型断言失败，期望string，实际: %T\n", testCaseMind)
		}
		return nil
	}

	if e.verbose {
		fmt.Printf("TestCaseMind字符串长度: %d\n", len(testCaseMindStr))
		fmt.Printf("TestCaseMind前100字符: %s\n", testCaseMindStr[:min(100, len(testCaseMindStr))])
		fmt.Printf("TestCaseMind后100字符: %s\n", testCaseMindStr[max(0, len(testCaseMindStr)-100):])

		// 检查字符串是否平衡
		openCount := strings.Count(testCaseMindStr, "{")
		closeCount := strings.Count(testCaseMindStr, "}")
		fmt.Printf("JSON括号平衡检查: 开括号{%d, 闭括号}%d\n", openCount, closeCount)

		// 检查字符串是否以{开始，以}结束
		if len(testCaseMindStr) > 0 {
			startsWithBrace := strings.HasPrefix(strings.TrimSpace(testCaseMindStr), "{")
			endsWithBrace := strings.HasSuffix(strings.TrimSpace(testCaseMindStr), "}")
			fmt.Printf("JSON格式检查: 以{开始:%v, 以}结束:%v\n", startsWithBrace, endsWithBrace)
		}
	}

	// 验证字符串完整性
	if len(testCaseMindStr) == 0 {
		if e.verbose {
			fmt.Println("TestCaseMind字符串为空")
		}
		return nil
	}

	// 解析TestCaseMind JSON字符串
	var testCaseMindData map[string]interface{}
	if err := json.Unmarshal([]byte(testCaseMindStr), &testCaseMindData); err != nil {
		if e.verbose {
			fmt.Printf("解析TestCaseMind JSON失败: %v\n", err)
			fmt.Printf("错误类型: %T\n", err)

			// 检查是否是unexpected end of JSON input错误
			if err.Error() == "unexpected end of JSON input" {
				fmt.Println("检测到'unexpected end of JSON input'错误，JSON可能被截断")
				// 尝试找到最后一个有效的位置
				lastValidPos := e.findLastValidJSONPosition(testCaseMindStr)
				fmt.Printf("最后有效JSON位置: %d\n", lastValidPos)
				if lastValidPos > 0 {
					fmt.Printf("截断的JSON片段: %s\n", testCaseMindStr[:lastValidPos])
				}
			}
		}
		return nil
	}

	if e.verbose {
		fmt.Println("JSON解析成功，TestCaseMind数据结构:")
		e.printJSONStructure(testCaseMindData, 0)
		fmt.Println("=== parseTestCaseMindStructureDirect 成功 ===")
	}

	// 使用结构模式识别
	return e.parseTestCaseMindStructurePattern(testCaseMindData)
}

// parseTestCaseMindStructurePattern 基于已知结构直接解析TestCaseMind
func (e *TreeExtractor) parseTestCaseMindStructurePattern(testCaseMindData map[string]interface{}) interface{} {
	if e.verbose {
		fmt.Println("开始解析TestCaseMind结构...")
	}

	// 检查是否有data字段
	if _, hasData := testCaseMindData["data"]; hasData {
		// 尝试解析根节点
		rootNode := e.parseTestCaseMindNode(testCaseMindData, 0)

		// 如果根节点为空但有children，则解析为多根结构
		if rootNode == nil {
			if childrenData, hasChildren := testCaseMindData["children"]; hasChildren {
				if childrenArray, ok := childrenData.([]interface{}); ok && len(childrenArray) > 0 {
					if e.verbose {
						fmt.Printf("解析为多根结构，共 %d 个顶级节点\n", len(childrenArray))
					}

					var validNodes []*SimplifiedNode
					for _, child := range childrenArray {
						if childMap, ok := child.(map[string]interface{}); ok {
							if candidate := e.parseTestCaseMindNode(childMap, 0); candidate != nil {
								validNodes = append(validNodes, candidate)
							}
						}
					}

					if len(validNodes) > 0 {
						if e.verbose {
							fmt.Printf("返回 %d 个有效根节点的数组\n", len(validNodes))
						}
						return validNodes
					}
				}
			}
		} else {
			// 成功解析出根节点，包装为数组格式保持一致性
			if e.verbose {
				fmt.Printf("检测到单根结构，根节点: %s\n", rootNode.Name)
			}
			return []*SimplifiedNode{rootNode}
		}
	}

	// 如果没有data字段但有children，尝试多根结构
	if childrenData, hasChildren := testCaseMindData["children"]; hasChildren {
		if childrenArray, ok := childrenData.([]interface{}); ok && len(childrenArray) > 0 {
			if e.verbose {
				fmt.Printf("检测到纯多根结构，共 %d 个顶级节点\n", len(childrenArray))
			}

			var validNodes []*SimplifiedNode
			for _, child := range childrenArray {
				if childMap, ok := child.(map[string]interface{}); ok {
					if candidate := e.parseTestCaseMindNode(childMap, 0); candidate != nil {
						validNodes = append(validNodes, candidate)
					}
				}
			}

			if len(validNodes) > 0 {
				return validNodes
			}
		}
	}

	if e.verbose {
		fmt.Println("未找到有效的TestCaseMind结构")
	}
	return nil
}



// extractTestCaseMindStructure 专门解析TestCaseMind的三层嵌套结构
func (e *TreeExtractor) extractTestCaseMindStructure(data interface{}) *SimplifiedNode {
	// 将数据转换为map以便访问
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	// 查找data字段
	dataField, exists := dataMap["data"]
	if !exists {
		return nil
	}

	dataMap2, ok := dataField.(map[string]interface{})
	if !ok {
		return nil
	}

	// 查找TestCaseMind字段
	testCaseMind, exists := dataMap2["TestCaseMind"]
	if !exists {
		return nil
	}

	testCaseMindStr, ok := testCaseMind.(string)
	if !ok {
		return nil
	}

	// 解析TestCaseMind JSON字符串
	var testCaseMindData map[string]interface{}
	if err := json.Unmarshal([]byte(testCaseMindStr), &testCaseMindData); err != nil {
		if e.verbose {
			fmt.Printf("解析TestCaseMind JSON失败: %v\n", err)
		}
		return nil
	}

	// 提取第一层：根节点的text
	rootData, ok := testCaseMindData["data"].(map[string]interface{})
	if !ok {
		return nil
	}

	rootText, ok := rootData["text"].(string)
	if !ok {
		return nil
	}

	// 检查根文本是否是业务文本
	if !e.isBusinessText(rootText) {
		return nil
	}

	// 创建根节点
	rootNode := &SimplifiedNode{
		Name: rootText,
		Children:  []*SimplifiedNode{},
	}

	// 提取第二层：children数组
	childrenData, exists := testCaseMindData["children"]
	if !exists {
		return rootNode
	}

	childrenArray, ok := childrenData.([]interface{})
	if !ok || len(childrenArray) == 0 {
		return rootNode
	}

	// 处理第一个子节点（第二层标题）
	firstChild, ok := childrenArray[0].(map[string]interface{})
	if !ok {
		return rootNode
	}

	firstChildData, ok := firstChild["data"].(map[string]interface{})
	if !ok {
		return rootNode
	}

	secondLevelText, ok := firstChildData["text"].(string)
	if !ok {
		return rootNode
	}

	// 检查二级标题是否是业务文本
	if !e.isBusinessText(secondLevelText) {
		return rootNode
	}

	// 创建第二层节点
	secondLevelNode := &SimplifiedNode{
		Name: secondLevelText,
		Children:  []*SimplifiedNode{},
	}

	// 提取第三层： grandchildren数组
	grandchildrenData, exists := firstChild["children"]
	if !exists {
		rootNode.Children = append(rootNode.Children, secondLevelNode)
		return rootNode
	}

	grandchildrenArray, ok := grandchildrenData.([]interface{})
	if !ok {
		rootNode.Children = append(rootNode.Children, secondLevelNode)
		return rootNode
	}

	// 处理第三层标题
	seen := make(map[string]bool)
	for _, grandchild := range grandchildrenArray {
		grandchildMap, ok := grandchild.(map[string]interface{})
		if !ok {
			continue
		}

		grandchildData, ok := grandchildMap["data"].(map[string]interface{})
		if !ok {
			continue
		}

		// 优先从richText中提取text
		if richTextArray, exists := grandchildData["richText"]; exists {
			if richTextItems, ok := richTextArray.([]interface{}); ok {
				for _, item := range richTextItems {
					if richTextObj, ok := item.(map[string]interface{}); ok {
						if textVal, textExists := richTextObj["text"]; textExists {
							if textStr, ok := textVal.(string); ok && textStr != "" && e.isBusinessText(textStr) && !seen[textStr] {
								thirdLevelNode := &SimplifiedNode{
									Name: textStr,
									Children:  []*SimplifiedNode{},
								}
								secondLevelNode.Children = append(secondLevelNode.Children, thirdLevelNode)
								seen[textStr] = true
							}
						}
					}
				}
			}
		}

		// 如果没有richText，则使用text字段
		if textVal, ok := grandchildData["text"].(string); ok && textVal != "" && e.isBusinessText(textVal) && !seen[textVal] {
			thirdLevelNode := &SimplifiedNode{
				Name: textVal,
				Children:  []*SimplifiedNode{},
			}
			secondLevelNode.Children = append(secondLevelNode.Children, thirdLevelNode)
			seen[textVal] = true
		}
	}

	// 使用递归解析器支持任意层级，直接解析整个结构
	rootNode = e.parseTestCaseMindNode(testCaseMindData, 0)

	if e.verbose && rootNode != nil {
		maxDepth := e.calculateTreeDepth(rootNode)
		fmt.Printf("成功解析TestCaseMind %d层嵌套结构，标题: %s，子节点数: %d\n", maxDepth, rootNode.Name, len(rootNode.Children))
	}

	return rootNode
}

// createGenericBusinessTextStructure 创建通用的业务文本结构（回退方案）
func (e *TreeExtractor) createGenericBusinessTextStructure(data interface{}) *SimplifiedNode {
	node := &SimplifiedNode{
		Children: []*SimplifiedNode{},
	}

	// 使用ExtractTextContent提取所有业务文本内容
	texts := e.ExtractTextContent(data)

	// 过滤掉技术字段，只保留真正的业务文本
	var businessTexts []string
	seen := make(map[string]bool) // 用于去重
	for _, text := range texts {
		if e.isBusinessText(text) && !seen[text] {
			businessTexts = append(businessTexts, text)
			seen[text] = true
		}
	}

	// 如果没有找到业务文本，使用默认标题
	if len(businessTexts) == 0 {
		node.Name = "API Response"
		return node
	}

	// 选择最长的文本作为标题（通常是最详细的业务描述）
	titleIndex := 0
	for i, text := range businessTexts {
		if len([]rune(text)) > len([]rune(businessTexts[titleIndex])) {
			titleIndex = i
		}
	}

	// 设置最长的文本作为标题
	node.Name = businessTexts[titleIndex]

	// 将其余业务文本作为子节点（按长度排序，长的在前）
	var childTexts []string
	for i, text := range businessTexts {
		if i != titleIndex {
			childTexts = append(childTexts, text)
		}
	}

	// 按文本长度降序排序子节点
	for i := 0; i < len(childTexts)-1; i++ {
		for j := i + 1; j < len(childTexts); j++ {
			if len([]rune(childTexts[i])) < len([]rune(childTexts[j])) {
				childTexts[i], childTexts[j] = childTexts[j], childTexts[i]
			}
		}
	}

	// 创建子节点
	for _, text := range childTexts {
		childNode := &SimplifiedNode{
			Name: text,
			Children:  []*SimplifiedNode{},
		}
		node.Children = append(node.Children, childNode)
	}

	if e.verbose {
		fmt.Printf("提取到 %d 个唯一业务文本，标题: %s\n", len(businessTexts), node.Name)
		fmt.Printf("子节点数量: %d\n", len(node.Children))
	}

	return node
}

// extractTree 递归抽取树结构
func (e *TreeExtractor) extractTree(obj map[string]interface{}, depth int) *SimplifiedNode {
	if depth > e.maxDepth {
		if e.verbose {
			fmt.Printf("警告: 达到最大递归深度 %d，停止递归\n", e.maxDepth)
		}
		return nil
	}

	node := &SimplifiedNode{
		Children: []*SimplifiedNode{},
	}

	// 1. 查找标题
	title := e.findTitle(obj)
	node.Name = title

	// 2. 查找子节点并递归
	children := e.findChildren(obj)
	for _, childData := range children {
		if childObj, ok := childData.(map[string]interface{}); ok {
			if childNode := e.extractTree(childObj, depth+1); childNode != nil {
				node.Children = append(node.Children, childNode)
			}
		}
	}

	// 3. 如果没有找到标准子节点，将所有嵌套对象作为子节点
	if len(node.Children) == 0 {
		for key, value := range obj {
			if key == title || value == nil {
				continue // 跳过标题字段和nil值
			}

			switch v := value.(type) {
			case map[string]interface{}:
				// 处理嵌套对象
				nestedNode := &SimplifiedNode{
					Name: fmt.Sprintf("%s (Object)", key),
					Children:  []*SimplifiedNode{},
				}

				for nestedKey, nestedValue := range v {
					if nestedStr, ok := nestedValue.(string); ok && nestedStr != "" {
						nestedChild := &SimplifiedNode{
							Name: fmt.Sprintf("%s: %s", nestedKey, nestedStr),
							Children:  []*SimplifiedNode{},
						}
						nestedNode.Children = append(nestedNode.Children, nestedChild)
					} else if nestedValue != nil {
						nestedChild := &SimplifiedNode{
							Name: fmt.Sprintf("%s: %v", nestedKey, nestedValue),
							Children:  []*SimplifiedNode{},
						}
						nestedNode.Children = append(nestedNode.Children, nestedChild)
					}
				}

				if len(nestedNode.Children) > 0 {
					node.Children = append(node.Children, nestedNode)
				}

			case []interface{}:
				// 处理数组
				arrayNode := &SimplifiedNode{
					Name: fmt.Sprintf("%s (Array - %d items)", key, len(v)),
					Children:  []*SimplifiedNode{},
				}

				for i, item := range v {
					if itemStr, ok := item.(string); ok && itemStr != "" {
						arrayChild := &SimplifiedNode{
							Name: fmt.Sprintf("[%d]: %s", i, itemStr),
							Children:  []*SimplifiedNode{},
						}
						arrayNode.Children = append(arrayNode.Children, arrayChild)
					} else if item != nil {
						arrayChild := &SimplifiedNode{
							Name: fmt.Sprintf("[%d]: %v", i, item),
							Children:  []*SimplifiedNode{},
						}
						arrayNode.Children = append(arrayNode.Children, arrayChild)
					}
				}

				if len(arrayNode.Children) > 0 {
					node.Children = append(node.Children, arrayNode)
				}
			}
		}
	}

	// 3. 只有当节点有标题或有子节点时才返回该节点
	if node.Name != "" || len(node.Children) > 0 {
		return node
	}

	return nil
}

// findTitle 查找节点标题
func (e *TreeExtractor) findTitle(obj map[string]interface{}) string {
	for _, key := range e.titleKeys {
		if value, exists := obj[key]; exists {
			if title, ok := value.(string); ok && title != "" {
				return title
			}
		}
	}
	return ""
}

// findChildren 查找子节点数组
func (e *TreeExtractor) findChildren(obj map[string]interface{}) []interface{} {
	for _, key := range e.childrenKeys {
		if value, exists := obj[key]; exists {
			// 检查是否为数组
			if reflect.TypeOf(value).Kind() == reflect.Slice {
				if children, ok := value.([]interface{}); ok && len(children) > 0 {
					return children
				}
			}
		}
	}
	return nil
}

// deepSearchInObject 深度搜索对象中的树结构
func (e *TreeExtractor) deepSearchInObject(obj map[string]interface{}) interface{} {
	// 检查当前对象是否有树结构
	if node := e.extractTree(obj, 0); node != nil {
		return node
	}

	// 常见的数据包装键
	dataKeys := []string{"data", "result", "response", "items", "list", "value"}

	// 递归搜索所有可能的嵌套结构
	return e.recursiveSearch(obj, dataKeys, 0)
}

// recursiveSearch 递归搜索树结构
func (e *TreeExtractor) recursiveSearch(data interface{}, keys []string, depth int) interface{} {
	if depth > e.maxDepth {
		return nil
	}

	switch v := data.(type) {
	case map[string]interface{}:
		// 检查当前对象是否是树结构
		if node := e.extractTree(v, depth); node != nil {
			return node
		}

		// 先尝试指定的键
		for _, key := range keys {
			if value, exists := v[key]; exists {
				if result := e.recursiveSearch(value, keys, depth+1); result != nil {
					return result
				}
			}
		}

		// 然后递归搜索所有值
		for _, value := range v {
			if result := e.recursiveSearch(value, keys, depth+1); result != nil {
				return result
			}
		}

	case []interface{}:
		// 搜索数组中的每个元素
		var roots []*SimplifiedNode
		for _, item := range v {
			if result := e.recursiveSearch(item, keys, depth+1); result != nil {
				if node, ok := result.(*SimplifiedNode); ok {
					roots = append(roots, node)
				} else if nodeArray, ok := result.([]*SimplifiedNode); ok {
					roots = append(roots, nodeArray...)
				}
			}
		}
		if len(roots) > 0 {
			return roots
		}
	}

	return nil
}

// searchInObject 在对象的值中搜索树结构
func (e *TreeExtractor) searchInObject(obj map[string]interface{}) interface{} {
	// 常见的数据包装键，如 "data", "result" 等
	dataKeys := []string{"data", "result", "response", "items", "list"}

	// 先尝试常见的数据键
	for _, key := range dataKeys {
		if value, exists := obj[key]; exists {
			if arr, ok := value.([]interface{}); ok {
				var roots []*SimplifiedNode
				for _, item := range arr {
					if childObj, ok := item.(map[string]interface{}); ok {
						if node := e.extractTree(childObj, 0); node != nil {
							roots = append(roots, node)
						}
					}
				}
				if len(roots) > 0 {
					return roots
				}
			} else if childObj, ok := value.(map[string]interface{}); ok {
				if node := e.extractTree(childObj, 0); node != nil {
					return node
				}
			}
		}
	}

	// 如果常见键没找到，遍历所有值
	for _, value := range obj {
		if arr, ok := value.([]interface{}); ok {
			var roots []*SimplifiedNode
			for _, item := range arr {
				if childObj, ok := item.(map[string]interface{}); ok {
					if node := e.extractTree(childObj, 0); node != nil {
						roots = append(roots, node)
					}
				}
			}
			if len(roots) > 0 {
				return roots
			}
		} else if childObj, ok := value.(map[string]interface{}); ok {
			if node := e.extractTree(childObj, 0); node != nil {
				return node
			}
		}
	}

	return nil
}

// SetMaxDepth 设置最大递归深度
func (e *TreeExtractor) SetMaxDepth(depth int) {
	e.maxDepth = depth
}

// GetStats 获取抽取统计信息
func (e *TreeExtractor) GetStats(data []byte) (map[string]interface{}, error) {
	var rawData interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	stats := make(map[string]interface{})

	switch v := rawData.(type) {
	case map[string]interface{}:
		stats["root_type"] = "object"
		stats["root_keys"] = e.getObjectKeys(v)
		e.collectStats(v, stats, "root")
	case []interface{}:
		stats["root_type"] = "array"
		stats["array_length"] = len(v)
		if len(v) > 0 {
			if firstObj, ok := v[0].(map[string]interface{}); ok {
				stats["first_item_keys"] = e.getObjectKeys(firstObj)
			}
		}
	}

	return stats, nil
}

// getObjectKeys 获取对象的所有键
func (e *TreeExtractor) getObjectKeys(obj map[string]interface{}) []string {
	var keys []string
	for key := range obj {
		keys = append(keys, key)
	}
	return keys
}

// collectStats 递归收集统计信息
func (e *TreeExtractor) collectStats(obj map[string]interface{}, stats map[string]interface{}, path string) {
	title := e.findTitle(obj)
	children := e.findChildren(obj)

	if title != "" {
		stats[path+"_has_title"] = true
	}

	if len(children) > 0 {
		stats[path+"_has_children"] = true
		stats[path+"_children_count"] = len(children)

		// 只检查前几个子节点的统计信息，避免过深
		maxCheck := min(3, len(children))
		for i := 0; i < maxCheck; i++ {
			if childObj, ok := children[i].(map[string]interface{}); ok {
				childPath := fmt.Sprintf("%s.child_%d", path, i)
				e.collectStats(childObj, stats, childPath)
			}
		}
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseTestCaseMindNode 直接解析TestCaseMind节点，基于已知结构
func (e *TreeExtractor) parseTestCaseMindNode(nodeData map[string]interface{}, depth int) *SimplifiedNode {
	if e.verbose {
		fmt.Printf("%sparseTestCaseMindNode 开始，深度: %d\n", strings.Repeat("  ", depth), depth)
	}

	// 防止无限递归
	if depth > e.maxDepth {
		if e.verbose {
			fmt.Printf("警告: 达到最大递归深度 %d，停止递归\n", e.maxDepth)
		}
		return nil
	}

	// 提取当前节点的数据
	currentData, ok := nodeData["data"].(map[string]interface{})
	if !ok {
		if e.verbose {
			fmt.Printf("%s未找到data字段或类型错误\n", strings.Repeat("  ", depth))
		}
		return nil
	}

	// 直接提取节点标题，不进行复杂的业务文本过滤
	var titleText string

	// 1. 优先从richText中提取第一个文本
	if richTextArray, exists := currentData["richText"]; exists {
		if richTextItems, ok := richTextArray.([]interface{}); ok && len(richTextItems) > 0 {
			for _, item := range richTextItems {
				if richTextObj, ok := item.(map[string]interface{}); ok {
					if textVal, ok := richTextObj["text"].(string); ok && textVal != "" {
						titleText = textVal
						break
					}
				}
			}
		}
	}

	// 2. 如果richText中没有，使用text字段
	if titleText == "" {
		if textVal, ok := currentData["text"].(string); ok && textVal != "" {
			titleText = textVal
		}
	}

	// 3. 对于根节点有子节点但无标题的情况，创建多根结构
	if titleText == "" && depth == 0 {
		childrenData, hasChildren := nodeData["children"]
		if hasChildren {
			if childrenArray, ok := childrenData.([]interface{}); ok && len(childrenArray) > 0 {
				// 直接返回多根结构数组
				var validNodes []*SimplifiedNode
				for _, child := range childrenArray {
					if childMap, ok := child.(map[string]interface{}); ok {
						if childNode := e.parseTestCaseMindNode(childMap, depth+1); childNode != nil {
							validNodes = append(validNodes, childNode)
						}
					}
				}
				if len(validNodes) > 0 {
					// 返回nil，让调用者处理多根结构
					return nil
				}
			}
		}
	}

	// 4. 如果还是没有标题，跳过这个节点
	if titleText == "" {
		if e.verbose {
			fmt.Printf("%s未找到有效标题，跳过节点\n", strings.Repeat("  ", depth))
		}
		return nil
	}

	// 创建当前节点
	simpleNode := &SimplifiedNode{
		Name: titleText,
		Children: []*SimplifiedNode{},
	}

	// 处理子节点
	childrenData, exists := nodeData["children"]
	if !exists {
		if e.verbose {
			fmt.Printf("%s无children字段，返回节点: '%s'\n", strings.Repeat("  ", depth), titleText)
		}
		return simpleNode
	}

	childrenArray, ok := childrenData.([]interface{})
	if !ok || len(childrenArray) == 0 {
		if e.verbose {
			fmt.Printf("%schildren为空，返回节点: '%s'\n", strings.Repeat("  ", depth), titleText)
		}
		return simpleNode
	}

	// 直接递归处理所有子节点
	for i, child := range childrenArray {
		childMap, ok := child.(map[string]interface{})
		if !ok {
			if e.verbose {
				fmt.Printf("%s子节点 %d 格式错误\n", strings.Repeat("  ", depth), i)
			}
			continue
		}

		childNode := e.parseTestCaseMindNode(childMap, depth+1)
		if childNode != nil {
			simpleNode.Children = append(simpleNode.Children, childNode)
		}
	}

	if e.verbose {
		fmt.Printf("%s完成节点解析: '%s', 子节点数: %d\n", strings.Repeat("  ", depth), titleText, len(simpleNode.Children))
	}

	return simpleNode
}

// parseMultiRootNode 解析多根节点结构
func (e *TreeExtractor) parseMultiRootNode(childrenArray []interface{}, depth int) interface{} {
	if e.verbose {
		fmt.Printf("%s=== parseMultiRootNode 开始，子节点数: %d ===\n", strings.Repeat("  ", depth), len(childrenArray))
	}

	var validNodes []*SimplifiedNode
	for i, child := range childrenArray {
		childMap, ok := child.(map[string]interface{})
		if !ok {
			if e.verbose {
				fmt.Printf("%s子节点 %d 格式错误\n", strings.Repeat("  ", depth), i)
			}
			continue
		}

		childNode := e.parseTestCaseMindNode(childMap, depth+1)
		if childNode != nil {
			if e.verbose {
				fmt.Printf("%s找到有效根节点 %d: '%s'\n", strings.Repeat("  ", depth), len(validNodes)+1, childNode.Name)
			}
			validNodes = append(validNodes, childNode)
		}
	}

	if e.verbose {
		fmt.Printf("%s=== parseMultiRootNode 完成，有效节点数: %d ===\n", strings.Repeat("  ", depth), len(validNodes))
	}

	if len(validNodes) > 0 {
		return validNodes // 返回数组表示多根结构
	}

	return nil
}

// selectBestBusinessRootNode 智能选择最合适的业务根节点
func (e *TreeExtractor) selectBestBusinessRootNode(nodes []*SimplifiedNode) *SimplifiedNode {
	if len(nodes) == 0 {
		return nil
	}
	if len(nodes) == 1 {
		return nodes[0]
	}

	if e.verbose {
		fmt.Println("开始智能选择最佳业务根节点...")
	}

	// 评分系统：为每个节点打分
	type scoredNode struct {
		node   *SimplifiedNode
		score  int
		reason string
	}

	var scoredNodes []scoredNode

	for _, node := range nodes {
		score := 0
		reasons := []string{}

		// 评分标准1: 优先选择包含"客户详情"、"门店列表"等关键词的节点
		nodeName := strings.ToLower(node.Name)
		priorityKeywords := []string{"客户详情", "门店列表", "详情", "列表"}
		for _, keyword := range priorityKeywords {
			if strings.Contains(nodeName, keyword) {
				score += 100
				reasons = append(reasons, fmt.Sprintf("包含优先关键词'%s'", keyword))
				break // 找到一个就足够了
			}
		}

		// 评分标准2: 避免选择包含"接口"、"系统"等技术性描述的节点
		avoidKeywords := []string{"接口", "系统", "平台", "验证", "测试"}  // 移除了业务相关的词汇
		for _, keyword := range avoidKeywords {
			if strings.Contains(nodeName, keyword) {
				score -= 50
				reasons = append(reasons, fmt.Sprintf("包含技术性关键词'%s'", keyword))
			}
		}

		// 评分标准3: 子节点数量（有子节点的优先）
		if len(node.Children) > 0 {
			score += 20
			reasons = append(reasons, fmt.Sprintf("有%d个子节点", len(node.Children)))
		}

		// 评分标准4: 文本长度适中
		textLength := len([]rune(node.Name))
		if textLength >= 4 && textLength <= 15 {
			score += 10
			reasons = append(reasons, "文本长度适中")
		}

		scoredNodes = append(scoredNodes, scoredNode{
			node:   node,
			score:  score,
			reason: strings.Join(reasons, ", "),
		})

		if e.verbose {
			fmt.Printf("节点 '%s': %d分 (%s)\n", node.Name, score, strings.Join(reasons, ", "))
		}
	}

	// 选择得分最高的节点
	best := scoredNodes[0]
	for _, scored := range scoredNodes {
		if scored.score > best.score {
			best = scored
		}
	}

	if e.verbose {
		fmt.Printf("最终选择: '%s' (%d分)\n", best.node.Name, best.score)
	}

	return best.node
}

// calculateTreeDepth 计算树的最大深度
func (e *TreeExtractor) calculateTreeDepth(node *SimplifiedNode) int {
	if node == nil {
		return 0
	}

	if len(node.Children) == 0 {
		return 1
	}

	maxChildDepth := 0
	for _, child := range node.Children {
		childDepth := e.calculateTreeDepth(child)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}

	return 1 + maxChildDepth
}

// findLastValidJSONPosition 找到最后一个有效的JSON位置
func (e *TreeExtractor) findLastValidJSONPosition(jsonStr string) int {
	bracketCount := 0
	inString := false
	escaped := false

	for i, char := range jsonStr {
		if escaped {
			escaped = false
			continue
		}

		switch char {
		case '\\':
			escaped = true
		case '"':
			inString = !inString
		case '{':
			if !inString {
				bracketCount++
			}
		case '}':
			if !inString {
				bracketCount--
				if bracketCount == 0 {
					return i + 1
				}
			}
		}
	}

	return 0
}

// printJSONStructure 打印JSON结构（调试用）
func (e *TreeExtractor) printJSONStructure(data interface{}, indent int) {
	if indent > 3 { // 限制深度避免过多输出
		return
	}

	prefix := strings.Repeat("  ", indent)

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			switch value.(type) {
			case map[string]interface{}, []interface{}:
				fmt.Printf("%s%s: (complex type)\n", prefix, key)
				if indent < 2 {
					e.printJSONStructure(value, indent+1)
				}
			default:
				if str, ok := value.(string); ok && len(str) > 50 {
					fmt.Printf("%s%s: \"%s...\" (length:%d)\n", prefix, key, str[:47], len(str))
				} else {
					fmt.Printf("%s%s: %v\n", prefix, key, value)
				}
			}
		}
	case []interface{}:
		fmt.Printf("%s(array with %d items)\n", prefix, len(v))
		if len(v) > 0 && indent < 2 {
			e.printJSONStructure(v[0], indent+1)
		}
	default:
		fmt.Printf("%s%v\n", prefix, v)
	}
}

// isUIBusinessText 专门判断是否为UI业务文本
func (e *TreeExtractor) isUIBusinessText(text string, depth int) bool {
	if text == "" {
		return false
	}

	// 常见的UI业务元素
	uiBusinessElements := []string{
		"APP端", "PC端", "Web端", "移动端", "桌面端",
		"接口验证", "筛选", "排序", "搜索", "列表", "详情",
		"展示", "页面", "界面", "菜单", "按钮", "选项",
		"指标", "数据", "统计", "报表", "图表",
		"功能", "模块", "组件", "流程",
	}

	for _, element := range uiBusinessElements {
		if strings.Contains(text, element) {
			return true
		}
	}

	// 对于特定的组合词汇，也认为是业务文本
	if strings.Contains(text, "&") || strings.Contains(text, "端") {
		// 检查是否包含其他业务关键词
		businessKeywords := []string{"客户", "门店", "订单", "商品", "用户", "数据", "接口"}
		for _, keyword := range businessKeywords {
			if strings.Contains(text, keyword) {
				return true
			}
		}
	}

	// 检查常见的业务动作和状态描述
	businessActions := []string{"点击", "页面", "其他", "内容", "手动", "打开", "状态", "为准", "不影响", "当前", "开关", "状态", "配置", "tcc", "引导", "收起", "助手", "自动"}
	for _, action := range businessActions {
		if strings.Contains(text, action) {
			if e.verbose {
				fmt.Printf("识别业务动作文本: '%s' (包含关键词: '%s')\n", text, action)
			}
			return true
		}
	}

	// 检查时间相关的业务文本（如秒、分钟等时间描述）
	if strings.Contains(text, "秒") || strings.Contains(text, "分钟") || strings.Contains(text, "后") {
		// 检查是否包含业务场景关键词
		timeBusinessKeywords := []string{"收起", "关闭", "隐藏", "消失", "展示", "显示", "提示", "引导", "助手", "页面", "自动"}
		for _, keyword := range timeBusinessKeywords {
			if strings.Contains(text, keyword) {
				if e.verbose {
					fmt.Printf("识别时间相关业务文本: '%s' (包含关键词: '%s')\n", text, keyword)
				}
				return true
			}
		}
	}

	// 检查埋点和数据统计相关的业务文本
	if strings.Contains(text, "埋点") || strings.Contains(text, "上报") || strings.Contains(text, "统计") || strings.Contains(text, "快捷筛选") {
		if e.verbose {
			fmt.Printf("识别埋点统计业务文本: '%s'\n", text)
		}
		return true
	}

	// 检查配置和开关相关的业务文本
	if strings.Contains(text, "配置") || strings.Contains(text, "开关") || strings.Contains(text, "tcc") || strings.Contains(text, "手动设置") {
		if e.verbose {
			fmt.Printf("识别配置开关业务文本: '%s'\n", text)
		}
		return true
	}

	// 检查BD操作相关的业务文本
	if strings.Contains(text, "BD") || strings.Contains(text, "bd") {
		bdActions := []string{"设置", "配置", "手动", "自动", "外呼", "开关", "状态", "页面", "���手"}
		for _, action := range bdActions {
			if strings.Contains(text, action) {
				if e.verbose {
					fmt.Printf("识别BD操作业务文本: '%s' (包含关键词: '%s')\n", text, action)
				}
				return true
			}
		}
	}

	// 特殊���合检查：识别常见的UI交互动作
	uiInteractions := []string{
		"点击页面其他", "点击其他", "页面其他内容", "其他内容",
		"手动打开", "打开状态", "开关状态", "tcc配置",
		"AI助手", "助手引导", "引导收起", "自动收起",
		"页面内容", "页面其他", "非按钮", "非输入框",
	}
	for _, interaction := range uiInteractions {
		if strings.Contains(text, interaction) {
			if e.verbose {
				fmt.Printf("识别UI交互文本: '%s' (匹配模式: '%s')\n", text, interaction)
			}
			return true
		}
	}

	// 检查是否为描述开关状态或配置相关的文本
	if (strings.Contains(text, "为准") && strings.Contains(text, "不影响")) ||
	   (strings.Contains(text, "手动") && strings.Contains(text, "状态")) ||
	   (strings.Contains(text, "配置") && strings.Contains(text, "tcc")) ||
	   (strings.Contains(text, "当前") && strings.Contains(text, "开关")) {
		if e.verbose {
			fmt.Printf("识别状态配置文本: '%s'\n", text)
		}
		return true
	}

	// 专门检查编号格式的业务文本
	if strings.HasPrefix(text, "1.") || strings.HasPrefix(text, "2.") || strings.HasPrefix(text, "3.") ||
	   strings.HasPrefix(text, "4.") || strings.HasPrefix(text, "5.") || strings.HasPrefix(text, "6.") ||
	   strings.HasPrefix(text, "7.") || strings.HasPrefix(text, "8.") || strings.HasPrefix(text, "9.") {
		// 检查是否包含业务关键词
		stepBusinessKeywords := []string{"用户", "查询", "指标", "数据", "结果", "展示",
			"Agent", "多轮", "对话", "携带", "上下文", "筛选", "条件", "切换", "主题", "开始", "新",
			"问题", "体验", "优化", "CRM", "智能", "数值", "空", "拒答", "场景", "历史", "存在", "维度"}
		for _, keyword := range stepBusinessKeywords {
			if strings.Contains(text, keyword) {
				if e.verbose {
					fmt.Printf("识别编号格式业务文本: '%s' (包含关键词: '%s')\n", text, keyword)
				}
				return true
			}
		}
	}

	return false
}

// inferTitleFromChildren 从子节点推断合适的标题
func (e *TreeExtractor) inferTitleFromChildren(childrenArray []interface{}, depth int) string {
	if e.verbose {
		fmt.Printf("%s开始从子节点推断标题，子节点数: %d\n", strings.Repeat("  ", depth), len(childrenArray))
	}

	// 收集所有子节点的名称
	var childNames []string
	for _, child := range childrenArray {
		if childMap, ok := child.(map[string]interface{}); ok {
			// 尝试从子节点的data中提取text
			if childData, hasData := childMap["data"]; hasData {
				if dataMap, ok := childData.(map[string]interface{}); ok {
					if textVal, hasText := dataMap["text"]; hasText {
						if textStr, ok := textVal.(string); ok && textStr != "" && e.isBusinessText(textStr) {
							childNames = append(childNames, textStr)
							if e.verbose {
								fmt.Printf("%s找到子节点文本: '%s'\n", strings.Repeat("  ", depth), textStr)
							}
						}
					}
					// 也检查richText
					if richTextArray, hasRichText := dataMap["richText"]; hasRichText {
						if richTextItems, ok := richTextArray.([]interface{}); ok {
							for _, item := range richTextItems {
								if richTextObj, ok := item.(map[string]interface{}); ok {
									if textVal, hasText := richTextObj["text"]; hasText {
										if textStr, ok := textVal.(string); ok && textStr != "" && e.isBusinessText(textStr) {
											childNames = append(childNames, textStr)
											if e.verbose {
												fmt.Printf("%s找到子节点richText: '%s'\n", strings.Repeat("  ", depth), textStr)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if len(childNames) == 0 {
		if e.verbose {
			fmt.Printf("%s未找到有效的子节点文本\n", strings.Repeat("  ", depth))
		}
		return ""
	}

	// 分析子节点名称的模式来推断父节点标题
	if e.verbose {
		fmt.Printf("%s子节点名称: %v\n", strings.Repeat("  ", depth), childNames)
	}

	// 模式1: 如果子节点都包含时间相关的词汇（如"3秒后"、"5秒后"），推断为时间相关的自动操作
	timeRelatedCount := 0
	for _, name := range childNames {
		if strings.Contains(name, "秒后") || strings.Contains(name, "分钟后") || strings.Contains(name, "自动收起") || strings.Contains(name, "自动关闭") {
			timeRelatedCount++
		}
	}
	if timeRelatedCount > 0 && timeRelatedCount == len(childNames) {
		return "自动收起/关闭"
	}

	// 模式2: 如果子节点包含"埋点"、"上报"、"筛选"等词汇，推断为数据埋点相关
	for _, name := range childNames {
		if strings.Contains(name, "埋点") || strings.Contains(name, "上报") || strings.Contains(name, "快捷筛选") {
			return "数据埋点与统计"
		}
	}

	// 模式3: 如果子节点包含配置、开关、tcc等词汇，推断为配置相关
	for _, name := range childNames {
		if strings.Contains(name, "配置") || strings.Contains(name, "开关") || strings.Contains(name, "tcc") || strings.Contains(name, "手动设置") {
			return "配置与开关状态"
		}
	}

	// 模式4: 如果子节点包含BD相关操作，推断为BD操作
	for _, name := range childNames {
		if strings.Contains(name, "BD") || strings.Contains(name, "手动设置") {
			return "BD手动配置与状态"
		}
	}

	// 模式5: 通用模式 - 从子节点名称中提取共同的关键词
	allText := strings.Join(childNames, " ")

	// 检查是否有明显的业务主题
	businessThemes := []struct {
		keywords []string
		title    string
	}{
		{[]string{"助手", "引导", "收起", "自动"}, "AI助手交互"},
		{[]string{"外呼", "开关", "配置"}, "自动外呼配置"},
		{[]string{"筛选", "埋点", "统计"}, "筛选埋点功能"},
		{[]string{"状态", "开关", "tcc"}, "状态管理"},
	}

	for _, theme := range businessThemes {
		matchCount := 0
		for _, keyword := range theme.keywords {
			if strings.Contains(allText, keyword) {
				matchCount++
			}
		}
		if matchCount >= 2 { // 至少匹配两个关键词
			return theme.title
		}
	}

	// 模式6: 如果所有模式都不匹配，返回第一个子节点的核心概念
	if len(childNames) > 0 {
	 firstName := childNames[0]
		// 提取前几个字符作为简化标题
		if len([]rune(firstName)) > 10 {
			return string([]rune(firstName)[:8]) + "..."
		}
		return firstName
	}

	return ""
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// marshalJSONWithoutEscape 自定义JSON序列化，处理Unicode转义
func marshalJSONWithoutEscape(v interface{}) ([]byte, error) {
	// 先使用标准的json.MarshalIndent进行序列化
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}

	// 先处理常见的Unicode转义序列为实际字符
	result := bytes.ReplaceAll(data, []byte("\\u0026"), []byte("&"))
	result = bytes.ReplaceAll(result, []byte("\\u003c"), []byte("<"))
	result = bytes.ReplaceAll(result, []byte("\\u003e"), []byte(">"))
	result = bytes.ReplaceAll(result, []byte("\\u0027"), []byte("'"))

	// 注意：不处理 \u0022 和 \u005c，因为它们会破坏JSON格式
	// 只处理非JSON结构性的Unicode转义

	// 然后使用更通用的方法：解码所有的Unicode转义序列
	result = decodeUnicodeEscapes(result)

	// 不处理字符串内的引号转义，以保持JSON格式有效性
	// 如果你需要更可读的输出，可以考虑生成其他格式（如纯文本）

	return result, nil
}


// decodeUnicodeEscapes 解码所有Unicode转义序列
func decodeUnicodeEscapes(data []byte) []byte {
	result := data
	i := 0

	for i < len(result) {
		// 查找Unicode转义序列的开始 \u
		if i+5 < len(result) && result[i] == '\\' && result[i+1] == 'u' {
			// 提取4位十六进制数
			hexStr := string(result[i+2 : i+6])

			// 解析十六进制数
			var r rune
			_, err := fmt.Sscanf(hexStr, "%04x", &r)
			if err == nil && utf8.ValidRune(r) {
				// 将rune编码为UTF-8字节
				utf8Bytes := make([]byte, 4)
				n := utf8.EncodeRune(utf8Bytes, r)
				utf8Bytes = utf8Bytes[:n]

				// 替换���义序列
				before := result[:i]
				after := result[i+6:]
				result = append(append(before, utf8Bytes...), after...)

				// 跳过新插入的UTF-8字节
				i += n
				continue
			}
		}
		i++
	}

	return result
}