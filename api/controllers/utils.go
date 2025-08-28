package controllers

import "strconv"

// parseIntWithDefault 解析整数参数，失败时返回默认值
func parseIntWithDefault(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return defaultValue
}
