package extractor

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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

	// 序列化结果
	output, err := json.MarshalIndent(result, "", "  ")
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

// isBusinessText 判断文本是否为业务文本（非技术字段）
func (e *TreeExtractor) isBusinessText(text string) bool {
	if text == "" {
		return false
	}

	// 过滤掉明显的技术字段和ID
	technicalKeywords := []string{
		"CreatedAt", "UpdatedAt", "TestCaseId", "ProductId", "errCode",
		"Status", "StatusCode", "CaseNum", "BaseType", "SourceType",
		"IsTemplate", "IsStrict", "Theme", "Remark", "Template",
		"EntityId", "EntityVersion", "CustomFields", "Directory",
		"CreatedBy", "UpdatedBy", "ParentDirLink", "BaseResp",
		"CreatedAtTS", "UpdatedAtTS", "TestCaseStatus", "CommentInfo",
		"BelongIDList", "MarkingCheck", "StrictMindVersion",
		"BitsDependencies", "TempParentDirLink", "FlowItemList",
		"ScriptCaseList", "images", "imageSize", "hyperlink",
		"hyperlinkTitle", "progress", "priority", "script_task",
		"resource", "nodeType", "parentID", "attachment", "genId",
		"WorkItemTypeKey", "RequirementId", "RequirementLink",
		"RequirementSource", "RequirementTitle", "project", "projectName",
		"UserKey", "DisplayName", "EnName", "DirId", "DirName",
		"DirNameEn", "total", "finish", "session_id", "expiry_time",
		"token_type", "permissions", "user", "donghe", "kilov",
		"sess_", "JWT", "debug_info", "details", "suggestions",
		"Please refresh", "Check your", "Contact support", "Auth",
		"ERROR", "validate", "failed", "expired", "Token",
		"王通", "张三", "李四", "wangtong", "Created By", "updated by",
		// 新增：过滤错误响应相关的技术词汇
		"message", "Auth ERROR", "Jwt validate failed", "API Response",
		"Unauthorized", "errCode", "caseApi", "getCaseDetail",
	}

	// 检查是否包含技术关键词
	for _, keyword := range technicalKeywords {
		if strings.Contains(text, keyword) {
			return false
		}
	}

	// 新增：特殊过滤 - 如果文本看起来像是API错误响应的一部分，直接过滤
	if strings.Contains(text, "Auth ERROR") || strings.Contains(text, "Jwt validate failed") ||
	   strings.Contains(text, "API Response") || strings.Contains(text, "errCode") {
		return false
	}

	// 过滤掉人名模式（通常是2-3个中文字符，后面可能跟点号）
	if len([]rune(text)) >= 2 && len([]rune(text)) <= 4 && hasChinese(text) {
		// 改进的人名检测 - 包含更多业务关键词
		businessKeywords := []string{"测试", "优化", "性能", "场景", "验证", "回归",
			"组", "实验", "对照", "逻辑", "方案", "功能", "页面", "模块", "流程",
			"门店", "搜索", "输入", "结果", "包含", "客户", "详情", "列表", "数据", "扫码", "核销",
			"资产", "中心", "商家", "产品", "实时", "订单", "指标", "展示", "排序", "筛选",
			"从高", "从低", "由远", "由近", "到大", "到小", "默认", "不可", "操作", "高到低", "低到高", "远到近", "近到远",
			// CRM和Agent相关词汇
			"CRM", "Agent", "智能", "对话", "多轮", "携带", "上下文", "切换", "主题", "问题", "体验", "优化",
			"查询", "数值", "空", "拒答", "场景", "历史", "存在", "维度", "正确", "展示为"}
		isBusiness := false
		for _, keyword := range businessKeywords {
			if strings.Contains(text, keyword) {
				isBusiness = true
				break
			}
		}
		if !isBusiness {
			return false // 很可能是人名
		}
	}

	// 检查是否为纯技术数据（如时间戳、ID、数字等），但要避免误判业务编号文本
	// 只有当文本以数字开头且长度很短时才认为是技术数据
	if (strings.HasPrefix(text, "1.") || strings.HasPrefix(text, "2.") ||
	    strings.HasPrefix(text, "3.") || strings.HasPrefix(text, "4.") ||
	    strings.HasPrefix(text, "5.") || strings.HasPrefix(text, "6.") ||
	    strings.HasPrefix(text, "7.") || strings.HasPrefix(text, "8.") ||
	    strings.HasPrefix(text, "9.")) && len([]rune(text)) < 10 {
		// 短的数字开头文本可能是业务步骤，检查是否包含业务关键词
		businessKeywords := []string{"用户", "查询", "指标", "数据", "结果", "展示",
			"Agent", "多轮", "对话", "携带", "上下文", "筛选", "条件", "切换", "主题", "开始", "新"}
		hasBusinessKeyword := false
		for _, keyword := range businessKeywords {
			if strings.Contains(text, keyword) {
				hasBusinessKeyword = true
				break
			}
		}
		if !hasBusinessKeyword {
			return false // 纯技术编号文本
		}
	}

	if strings.HasPrefix(text, "e+") || strings.HasPrefix(text, "E+") ||
	   strings.HasPrefix(text, "[]") || strings.HasPrefix(text, "{}") ||
	   strings.HasPrefix(text, "map[") || strings.Contains(text, ": 0") ||
	   strings.Contains(text, ": 1") || strings.Contains(text, ": false") ||
	   strings.Contains(text, ": true") || strings.Contains(text, "read write") {
		return false
	}

	// 过滤掉短英文技术词汇
	shortTechnicalWords := []string{"api", "url", "http", "get", "post", "put", "delete"}
	for _, word := range shortTechnicalWords {
		if strings.EqualFold(text, word) {
			return false
		}
	}

	// 检查是否为中文或英文业务文本
	if hasChinese(text) {
		return true
	}

	// 英文文本需要更长且包含业务词汇
	if isEnglishBusinessText(text) {
		return true
	}

	return false
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

// createDefaultStructure 为非标准响应创建默认树状结构，只提取业务文本
func (e *TreeExtractor) createDefaultStructure(data interface{}) interface{} {
	if e.verbose {
		fmt.Println("创建默认树状结构...")
	}

	// 优先尝试解析TestCaseMind结构
	if testCaseMindNodes := e.parseTestCaseMindStructureDirect(data); testCaseMindNodes != nil {
		if e.verbose {
			fmt.Println("成功解析TestCaseMind结构")
		}
		return testCaseMindNodes
	}

	// 然后尝试标准的树结构解析
	if standardTree := e.tryStandardTreeStructure(data); standardTree != nil {
		if e.verbose {
			fmt.Println("成功解析标准树结构")
		}
		return standardTree
	}

	// 回退到通用的业务文本提取
	return e.createGenericBusinessTextStructure(data)
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

// parseTestCaseMindStructurePattern 基于JSON结构模式识别来解析TestCaseMind
func (e *TreeExtractor) parseTestCaseMindStructurePattern(testCaseMindData map[string]interface{}) interface{} {
	if e.verbose {
		fmt.Println("开始结构模式识别...")
	}

	// 检查是否有data字段
	if _, hasData := testCaseMindData["data"]; hasData {
		// 尝试解析根节点
		rootNode := e.parseTestCaseMindNode(testCaseMindData, 0)

		// 如果根节点为空（比如text为空），但有children，则解析为多根结构
		if rootNode == nil {
			if childrenData, hasChildren := testCaseMindData["children"]; hasChildren {
				if childrenArray, ok := childrenData.([]interface{}); ok && len(childrenArray) > 0 {
					if e.verbose {
						fmt.Printf("根节点text为空，解析为多根结构，共 %d 个顶级节点\n", len(childrenArray))
					}

					var validNodes []*SimplifiedNode
					for _, child := range childrenArray {
						childMap, ok := child.(map[string]interface{})
						if !ok {
							continue
						}

						if candidate := e.parseTestCaseMindNode(childMap, 0); candidate != nil {
							if e.verbose {
								fmt.Printf("找到第 %d 个有效根节点: %s\n", len(validNodes)+1, candidate.Name)
							}
							validNodes = append(validNodes, candidate)
						}
					}

					if len(validNodes) > 0 {
						if e.verbose {
							fmt.Printf("返回 %d 个有效根节点的数组\n", len(validNodes))
						}
						// 返回数组格式，与预期结果一致
						return validNodes
					}

					if e.verbose {
						fmt.Println("没有找到有效的根节点")
					}
				}
			}
		} else {
			// 成功解析出根节点，检查是否需要转换为数组格式
			if e.verbose {
				fmt.Printf("检测到标准单根结构，根节点: %s\n", rootNode.Name)
			}

			// 根据预期结果，将单根节点也包装成数组格式
			// 这样保持输出格式的一致性
			return []*SimplifiedNode{rootNode}
		}
	}

	// 检测是否为只有children数组的多根结构
	if childrenData, hasChildren := testCaseMindData["children"]; hasChildren {
		if childrenArray, ok := childrenData.([]interface{}); ok && len(childrenArray) > 0 {
			if e.verbose {
				fmt.Printf("检测到纯多根结构，共 %d 个顶级节点\n", len(childrenArray))
			}

			var validNodes []*SimplifiedNode
			for _, child := range childrenArray {
				childMap, ok := child.(map[string]interface{})
				if !ok {
					continue
				}

				if candidate := e.parseTestCaseMindNode(childMap, 0); candidate != nil {
					if e.verbose {
						fmt.Printf("找到第 %d 个有效根节点: %s\n", len(validNodes)+1, candidate.Name)
					}
					validNodes = append(validNodes, candidate)
				}
			}

			if len(validNodes) > 0 {
				if e.verbose {
					fmt.Printf("返回 %d 个有效根节点的数组\n", len(validNodes))
				}
				return validNodes
			}

			if e.verbose {
				fmt.Println("没有找到有效的根节点")
			}
		}
	}

	// 回退到原始解析
	if e.verbose {
		fmt.Println("回退到原始解析逻辑")
	}
	result := e.parseTestCaseMindNode(testCaseMindData, 0)

	// 如果根节点解析失败但存在children，尝试解析为多根结构
	if result == nil {
		if childrenData, hasChildren := testCaseMindData["children"]; hasChildren {
			if childrenArray, ok := childrenData.([]interface{}); ok && len(childrenArray) > 0 {
				if e.verbose {
					fmt.Printf("根节点解析失败，尝试多根结构解析，子节点数: %d\n", len(childrenArray))
				}
				return e.parseMultiRootNode(childrenArray, 0)
			}
		}
	} else {
		// 如果成功解析出根节点，将其包装为数组格式
		return []*SimplifiedNode{result}
	}

	return result
}

// isGoodRootNode 评估节点是否适合作为根节点
func (e *TreeExtractor) isGoodRootNode(node *SimplifiedNode) bool {
	if node == nil || node.Name == "" {
		return false
	}

	// 检查文本长度 - 根节点通常不要太长也不要太短
	textLength := len([]rune(node.Name))
	if textLength < 2 || textLength > 50 {
		if e.verbose {
			fmt.Printf("节点 '%s' 长度不合适: %d\n", node.Name, textLength)
		}
		return false
	}

	// 检查是否是真正的业务文本
	if !e.isBusinessText(node.Name) {
		if e.verbose {
			fmt.Printf("节点 '%s' 不符合业务文本特征\n", node.Name)
		}
		return false
	}

	// 检查是否包含过多的技术词汇
	technicalPatterns := []string{
		"接口", "系统", "平台", "验证", "测试",  // 移除了可能在业务标题中出现的词汇
		"API", "HTTP", "JSON", "XML", "SQL", "UI", "UX", "QA", "CI", "CD",
	}

	technicalCount := 0
	for _, pattern := range technicalPatterns {
		if strings.Contains(node.Name, pattern) {
			technicalCount++
		}
	}

	// 如果技术词汇占比过高（超过30%），可能不是好的根节点
	words := strings.Fields(node.Name)
	if len(words) > 0 && float64(technicalCount)/float64(len(words)) > 0.3 {
		if e.verbose {
			fmt.Printf("节点 '%s' 技术词汇过多: %d/%d\n", node.Name, technicalCount, len(words))
		}
		return false
	}

	// 检查是否是典型的业务场景描述
	businessKeywords := []string{
		"客户", "用户", "订单", "商品", "门店", "页面", "功能", "模块", "流程",
		"详情", "列表", "搜索", "添加", "编辑", "删除", "查看", "管理",
	}

	hasBusinessKeyword := false
	for _, keyword := range businessKeywords {
		if strings.Contains(node.Name, keyword) {
			hasBusinessKeyword = true
			break
		}
	}

	if !hasBusinessKeyword {
		if e.verbose {
			fmt.Printf("节点 '%s' 缺少业务关键词\n", node.Name)
		}
		return false
	}

	return true
}

// selectBestRootNode 从候选节点中选择最佳根节点
func (e *TreeExtractor) selectBestRootNode(candidates []*SimplifiedNode) *SimplifiedNode {
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	// 为每个候选节点评分
	type scoredNode struct {
		node   *SimplifiedNode
		score  float64
		reason string
	}

	var scoredNodes []scoredNode

	for _, candidate := range candidates {
		score := 0.0
		reasons := []string{}

		// 评分标准1: 文本长度适中（6-15个字符最佳）
		length := len([]rune(candidate.Name))
		if length >= 6 && length <= 15 {
			score += 30
			reasons = append(reasons, "长度适中")
		} else if length > 15 {
			score -= 10
			reasons = append(reasons, "过长")
		} else {
			score -= 20
			reasons = append(reasons, "过短")
		}

		// 评分标准2: 包含核心业务关键词
		coreKeywords := []string{"客户", "用户", "订单", "商品", "门店", "页面", "详情", "列表"}
		for _, keyword := range coreKeywords {
			if strings.Contains(candidate.Name, keyword) {
				score += 25
				reasons = append(reasons, fmt.Sprintf("包含核心关键词'%s'", keyword))
			}
		}

		// 评分标准3: 避免技术词汇
		technicalWords := []string{"系统", "平台", "接口", "验证", "测试"}  // 移除了业务相关的词汇
		technicalCount := 0
		for _, word := range technicalWords {
			if strings.Contains(candidate.Name, word) {
				technicalCount++
				score -= 15
			}
		}
		if technicalCount > 0 {
			reasons = append(reasons, fmt.Sprintf("包含%d个技术词汇", technicalCount))
		}

		// 评分标准4: 子节点数量（有子节点更好）
		if len(candidate.Children) > 0 {
			score += 20
			reasons = append(reasons, fmt.Sprintf("有%d个子节点", len(candidate.Children)))
		}

		// 评分标准5: 结构简洁性
		if !strings.Contains(candidate.Name, "-") || strings.Count(candidate.Name, "-") <= 1 {
			score += 10
			reasons = append(reasons, "结构简洁")
		}

		scoredNodes = append(scoredNodes, scoredNode{
			node:   candidate,
			score:  score,
			reason: strings.Join(reasons, ", "),
		})
	}

	// 选择得分最高的节点
	best := scoredNodes[0]
	for _, scored := range scoredNodes {
		if scored.score > best.score {
			best = scored
		}
	}

	if e.verbose {
		fmt.Printf("根节点选择结果:\n")
		for _, scored := range scoredNodes {
			marker := " "
			if scored.node.Name == best.node.Name {
				marker = "✓"
			}
			fmt.Printf("  %s '%s': %.1f分 (%s)\n", marker, scored.node.Name, scored.score, scored.reason)
		}
	}

	return best.node
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

// parseTestCaseMindNode 递归解析TestCaseMind节点，支持任意层级
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

	// 提取节点标题，优先从richText获取
	var titleText string

	// 优先从richText中提取标题
	if richTextArray, exists := currentData["richText"]; exists {
		if richTextItems, ok := richTextArray.([]interface{}); ok {
			if e.verbose {
				fmt.Printf("%s找到richText数组，长度: %d\n", strings.Repeat("  ", depth), len(richTextItems))
			}
			// 收集所有有效的业务文本
			var validTexts []string
			for _, item := range richTextItems {
				if richTextObj, ok := item.(map[string]interface{}); ok {
					if textVal, textExists := richTextObj["text"]; textExists {
						if textStr, ok := textVal.(string); ok && textStr != "" {
							if e.verbose {
								fmt.Printf("%srichText文本: '%s', 是否业务文本: %v\n", strings.Repeat("  ", depth), textStr, e.isBusinessText(textStr))
							}
							if e.isBusinessText(textStr) {
								validTexts = append(validTexts, textStr)
							}
						}
					}
				}
			}
			// 使用第一个有效的业务文本作为标题
			if len(validTexts) > 0 {
				titleText = validTexts[0]
				if e.verbose {
					fmt.Printf("%s使用richText作为标题: '%s'\n", strings.Repeat("  ", depth), titleText)
				}
			}
		}
	}

	// 如果richText中没有找到合适的标题，使用text字段
	if titleText == "" {
		if textVal, ok := currentData["text"].(string); ok {
			if e.verbose {
				fmt.Printf("%s发现text字段: '%s', 长度: %d\n", strings.Repeat("  ", depth), textVal, len(textVal))
			}
			// 对于根节点，如果text为空但有children，不直接返回nil
			if textVal != "" {
				// 放宽业务文本判断，特别是对于常见的业务界面元素
				if e.isBusinessText(textVal) || e.isUIBusinessText(textVal, depth) {
					titleText = textVal
					if e.verbose {
						fmt.Printf("%s使用text字段作为标题: '%s'\n", strings.Repeat("  ", depth), titleText)
					}
				} else if e.verbose {
					fmt.Printf("%stext字段不是业务文本，跳过: '%s'\n", strings.Repeat("  ", depth), textVal)
				}
			}
		}
	}

	// 对于有children的根节点，即使没有有效的标题，也尝试创建一个虚拟节点
	if titleText == "" {
		childrenData, hasChildren := nodeData["children"]
		if hasChildren {
			if childrenArray, ok := childrenData.([]interface{}); ok && len(childrenArray) > 0 {
				if depth == 0 {
					// 这是根节点且有子节点，为多根结构创建数组而不是单个节点
					if e.verbose {
						fmt.Printf("%s根节点无标题但有子节点，解析为多根结构\n", strings.Repeat("  ", depth))
					}
					// 继续解析子节点，让调用者处理多根结构，但不直接返回nil
					// 先尝试解析所有子节点，看看能否找到有效的根节点候选
					var validNodes []*SimplifiedNode
					for _, child := range childrenArray {
						if childMap, ok := child.(map[string]interface{}); ok {
							if childNode := e.parseTestCaseMindNode(childMap, depth+1); childNode != nil {
								validNodes = append(validNodes, childNode)
							}
						}
					}

					// 如果找到了有效的子节点，选择最佳的一个作为根节点
					if len(validNodes) > 0 {
						bestNode := e.selectBestBusinessRootNode(validNodes)
						if bestNode != nil {
							if e.verbose {
								fmt.Printf("%s从子节点中选择最佳根节点: '%s'\n", strings.Repeat("  ", depth), bestNode.Name)
							}
							return bestNode
						}
					}

					// 如果没有找到合适的子节点，则返回nil让调用者处理多根结构
					return nil
				} else {
					// 非根节点，使用默认标题
					titleText = "未命名节点"
					if e.verbose {
						fmt.Printf("%s使用默认标题: '%s'\n", strings.Repeat("  ", depth), titleText)
					}
				}
			}
		}
	}

	// 如果仍然没有找到标题，跳过这个节点
	if titleText == "" {
		if e.verbose {
			fmt.Printf("%s未找到有效标题，跳过节点\n", strings.Repeat("  ", depth))
		}
		return nil
	}

	// 创建当前节点
	simpleNode := &SimplifiedNode{
		Name: titleText,
		Children:  []*SimplifiedNode{},
	}

	// 递归处理子节点
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
			fmt.Printf("%schildren为空或格式错误，返回节点: '%s'\n", strings.Repeat("  ", depth), titleText)
		}
		return simpleNode
	}

	if e.verbose {
		fmt.Printf("%s处理 %d 个子节点\n", strings.Repeat("  ", depth), len(childrenArray))
	}

	// 处理每个子节点
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
				fmt.Printf("%s添加子节点: '%s'\n", strings.Repeat("  ", depth), childNode.Name)
			}
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

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}