package parser

import (
	"regexp"
	"strings"

	"caseurl2md/internal/config"
)

// parseCookies 解析 -b 或 --cookie 参数
func parseCookies(curlCmd string, info *config.RequestInfo) {
	// 使用正则表达式匹配 -b 或 --cookie 参数
	cookieRe := regexp.MustCompile(`(?:-b|--cookie)\s+['"]?([^'"\\]*(?:\\.[^'"\\]*)*)['"]?`)
	matches := cookieRe.FindAllStringSubmatch(curlCmd, -1)

	for _, match := range matches {
		if len(match) > 1 {
			cookieStr := match[1]
			// 移除可能的引号
			cookieStr = strings.Trim(cookieStr, `"'`)

			// 解析cookie字符串，格式为: key1=value1; key2=value2
			cookies := strings.Split(cookieStr, ";")
			for _, cookie := range cookies {
				cookie = strings.TrimSpace(cookie)
				if cookie == "" {
					continue
				}

				// 分割键值对
				parts := strings.SplitN(cookie, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					if key != "" {
						info.Cookies[key] = value
					}
				}
			}
		}
	}
}