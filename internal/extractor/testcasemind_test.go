package extractor

import (
	"encoding/json"
	"testing"
)

func TestTreeExtractor_TestCaseMind(t *testing.T) {
	extractor := New([]string{"case_title", "title", "name"}, []string{"children", "items", "nodes"}, false)

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
		expectArray bool
		expectedNames []string
	}{
		{
			name: "TestCaseMind单根结构",
			data: []byte(`{
				"data": {
					"TestCaseMind": "{\"data\":{\"text\":\"客户详情-门店列表\"},\"children\":[{\"data\":{\"text\":\"门店搜索\"},\"children\":[{\"data\":{\"richText\":[{\"text\":\"输入存在的门店名称\",\"type\":1}]},\"children\":[]}]}]}"
				}
			}`),
			wantErr: false,
			expectArray: false,
			expectedNames: []string{"客户详情-门店列表", "门店搜索", "输入存在的门店名称"},
		},
		{
			name: "TestCaseMind多根结构",
			data: []byte(`{
				"data": {
					"TestCaseMind": "{\"children\":[{\"data\":{\"text\":\"客户详情-门店列表\"},\"children\":[{\"data\":{\"richText\":[{\"text\":\"输入存在的门店名称\",\"type\":1}]},\"children\":[]}]}]}"
				}
			}`),
			wantErr: false,
			expectArray: true,
			expectedNames: []string{"客户详情-门店列表", "输入存在的门店名称"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractor.Extract(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Extract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// 解析结果
			var resultJSON interface{}
			if err := json.Unmarshal(got, &resultJSON); err != nil {
				t.Errorf("Extract() got invalid JSON: %v", err)
				return
			}

			// 检查是否为数组或单个对象
			if tt.expectArray {
				resultArray, ok := resultJSON.([]interface{})
				if !ok {
					t.Errorf("Expected array result, got %T", resultJSON)
					return
				}
				if len(resultArray) == 0 {
					t.Errorf("Expected non-empty array result")
					return
				}

				// 验证节点名称
				var foundNames []string
				for _, item := range resultArray {
					if node, ok := item.(map[string]interface{}); ok {
						if name, ok := node["name"].(string); ok {
							foundNames = append(foundNames, name)
						}
						// 检查子节点
						if children, ok := node["children"].([]interface{}); ok {
							for _, child := range children {
								if childNode, ok := child.(map[string]interface{}); ok {
									if childName, ok := childNode["name"].(string); ok {
										foundNames = append(foundNames, childName)
									}
								}
							}
						}
					}
				}

				for _, expectedName := range tt.expectedNames {
					found := false
					for _, foundName := range foundNames {
						if foundName == expectedName {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected to find name '%s' in result, but didn't. Found names: %v", expectedName, foundNames)
					}
				}
			} else {
				resultMap, ok := resultJSON.(map[string]interface{})
				if !ok {
					t.Errorf("Expected single object result, got %T", resultJSON)
					return
				}

				// 验证节点名称
				var foundNames []string
				collectNames(resultMap, &foundNames)

				for _, expectedName := range tt.expectedNames {
					found := false
					for _, foundName := range foundNames {
						if foundName == expectedName {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected to find name '%s' in result, but didn't. Found names: %v", expectedName, foundNames)
					}
				}
			}
		})
	}
}

// collectNames 递归收集树中所有节点的名称
func collectNames(node map[string]interface{}, names *[]string) {
	if name, ok := node["name"].(string); ok {
		*names = append(*names, name)
	}

	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			if childNode, ok := child.(map[string]interface{}); ok {
				collectNames(childNode, names)
			}
		}
	}
}