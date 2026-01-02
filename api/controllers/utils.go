package controllers

import (
	"net/http"
	"strconv"

	"box-manage-service/api/middleware"
)

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

// getUserIDFromRequest 从请求上下文中获取用户ID
// 如果未找到用户信息，返回默认用户ID 1（系统用户）
func getUserIDFromRequest(r *http.Request) uint {
	return middleware.GetUserIDFromContext(r.Context())
}

// getUserFromRequest 从请求上下文中获取完整用户信息
func getUserFromRequest(r *http.Request) *middleware.UserInfo {
	return middleware.GetUserFromContext(r.Context())
}
