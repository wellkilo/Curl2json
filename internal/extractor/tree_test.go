package extractor

import (
	"encoding/json"
	"testing"
)

func TestTreeExtractor_Extract(t *testing.T) {
	extractor := New([]string{"case_title", "title", "name"}, []string{"children", "items", "nodes"}, false)

	tests := []struct {
		name    string
		data    []byte
		want    string
		wantErr bool
	}{
		{
			name: "简单树结构",
			data: []byte(`{
				"case_title": "根节点",
				"children": [
					{"case_title": "子节点1", "children": []},
					{"case_title": "子节点2", "children": []}
				]
			}`),
			want: `{
  "case_title": "根节点",
  "children": [
    {
      "case_title": "子节点1",
      "children": []
    },
    {
      "case_title": "子节点2",
      "children": []
    }
  ]
}`,
			wantErr: false,
		},
		{
			name: "使用不同字段名",
			data: []byte(`{
				"title": "项目A",
				"items": [
					{"name": "功能1", "nodes": []}
				]
			}`),
			want: `{
  "case_title": "项目A",
  "children": [
    {
      "case_title": "功能1",
      "children": []
    }
  ]
}`,
			wantErr: false,
		},
		{
			name: "嵌套结构",
			data: []byte(`{
				"case_title": "根",
				"children": [
					{
						"case_title": "子1",
						"children": [
							{"case_title": "孙子1", "children": []}
						]
					}
				]
			}`),
			want: `{
  "case_title": "根",
  "children": [
    {
      "case_title": "子1",
      "children": [
        {
          "case_title": "孙子1",
          "children": []
        }
      ]
    }
  ]
}`,
			wantErr: false,
		},
		{
			name:    "无效JSON",
			data:    []byte(`{invalid json}`),
			want:    "",
			wantErr: true,
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

			// 比较JSON结构（忽略空格）
			var gotJSON, wantJSON interface{}
			if err := json.Unmarshal(got, &gotJSON); err != nil {
				t.Errorf("Extract() got invalid JSON: %v", err)
				return
			}
			if err := json.Unmarshal([]byte(tt.want), &wantJSON); err != nil {
				t.Errorf("Extract() want invalid JSON: %v", err)
				return
			}

			gotBytes, _ := json.Marshal(gotJSON)
			wantBytes, _ := json.Marshal(wantJSON)

			if string(gotBytes) != string(wantBytes) {
				t.Errorf("Extract() = %v, want %v", string(gotBytes), string(wantBytes))
			}
		})
	}
}

func TestTreeExtractor_findTitle(t *testing.T) {
	extractor := New([]string{"case_title", "title", "name", "label"}, []string{"children"}, false)

	tests := []struct {
		name     string
		obj      map[string]interface{}
		expected string
	}{
		{
			name: "找到case_title",
			obj: map[string]interface{}{
				"case_title": "测试标题",
				"title":      "其他标题",
			},
			expected: "测试标题",
		},
		{
			name: "找到title",
			obj: map[string]interface{}{
				"name":  "名称",
				"title": "标题",
			},
			expected: "标题",
		},
		{
			name: "找到name",
			obj: map[string]interface{}{
				"name": "名称",
			},
			expected: "名称",
		},
		{
			name: "未找到标题",
			obj: map[string]interface{}{
				"other": "其他字段",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.findTitle(tt.obj)
			if result != tt.expected {
				t.Errorf("findTitle() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTreeExtractor_findChildren(t *testing.T) {
	extractor := New([]string{"title"}, []string{"children", "items", "nodes"}, false)

	tests := []struct {
		name     string
		obj      map[string]interface{}
		expected []interface{}
	}{
		{
			name: "找到children",
			obj: map[string]interface{}{
				"children": []interface{}{"child1", "child2"},
				"items":    []interface{}{"item1"},
			},
			expected: []interface{}{"child1", "child2"},
		},
		{
			name: "找到items",
			obj: map[string]interface{}{
				"items": []interface{}{"item1", "item2"},
			},
			expected: []interface{}{"item1", "item2"},
		},
		{
			name: "找到nodes",
			obj: map[string]interface{}{
				"nodes": []interface{}{"node1"},
			},
			expected: []interface{}{"node1"},
		},
		{
			name: "未找到子节点",
			obj: map[string]interface{}{
				"other": "其他字段",
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.findChildren(tt.obj)
			if len(result) != len(tt.expected) {
				t.Errorf("findChildren() length = %v, want %v", len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("findChildren()[%d] = %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}