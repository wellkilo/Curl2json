package extractor

import (
	"testing"
)

// TestIsUIBusinessText 测试UI业务文本识别
func TestIsUIBusinessText(t *testing.T) {
	e := New([]string{}, []string{}, false)

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "点击页面其他内容",
			text:     "点击页面其他内容",
			expected: true,
		},
		{
			name:     "以手动打开状态为准，tcc配置不影响当前开关状态",
			text:     "以手动打开状态为准，tcc配置不影响当前开关状态",
			expected: true,
		},
		{
			name:     "AI助手引导收起",
			text:     "AI助手引导收起",
			expected: true,
		},
		{
			name:     "AI助手自动收起",
			text:     "AI助手自动收起",
			expected: true,
		},
		{
			name:     "开发进度快捷筛选埋点",
			text:     "开发进度快捷筛选埋点",
			expected: true,
		},
		{
			name:     "1. BD在AI助手页面手动设置自动外呼开关状态",
			text:     "1. BD在AI助手页面手动设置自动外呼开关状态",
			expected: true,
		},
		{
			name:     "页面其他内容",
			text:     "页面其他内容",
			expected: true,
		},
		{
			name:     "点击页面其他",
			text:     "点击页面其他",
			expected: true,
		},
		{
			name:     "配置tcc",
			text:     "配置tcc",
			expected: true,
		},
		{
			name:     "CreatedAt",
			text:     "CreatedAt",
			expected: false,
		},
		{
			name:     "王通",
			text:     "王通",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 使用反射来调用私有方法进行测试
			result := e.isUIBusinessText(tt.text, 0)
			if result != tt.expected {
				t.Errorf("isUIBusinessText(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

// TestIsBusinessText 测试业务文本识别
func TestIsBusinessText(t *testing.T) {
	e := New([]string{}, []string{}, false)

	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "点击页面其他内容",
			text:     "点击页面其他内容",
			expected: true,
		},
		{
			name:     "以手动打开状态为准，tcc配置不影响当前开关状态",
			text:     "以手动打开状态为准，tcc配��不影响当前开关状态",
			expected: true,
		},
		{
			name:     "AI助手引导收起",
			text:     "AI助手引导收起",
			expected: true,
		},
		{
			name:     "1. BD在AI助手页面手动设置自动外呼开关状态",
			text:     "1. BD在AI助手页面手动设置自动外呼开关状态",
			expected: true,
		},
		{
			name:     "status",
			text:     "status",
			expected: false,
		},
		{
			name:     "errCode",
			text:     "errCode",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.isBusinessText(tt.text)
			if result != tt.expected {
				t.Errorf("isBusinessText(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}