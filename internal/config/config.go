package config

import "time"

// Config 工具配置
type Config struct {
	Timeout      time.Duration
	TitleKeys    []string
	ChildrenKeys []string
	Verbose      bool
}

// RequestInfo HTTP请求信息
type RequestInfo struct {
	URL     string
	Method  string
	Headers map[string]string
	Cookies map[string]string
	Body    string
}