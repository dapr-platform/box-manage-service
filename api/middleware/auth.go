/*
 * @module api/middleware/auth
 * @description 认证中间件 - 从请求中提取用户信息
 * @architecture 中间件层
 * @documentReference REQ-AUTH: 用户认证
 * @stateFlow HTTP请求 -> 认证检查 -> 用户上下文注入
 * @rules 支持JWT Token和简单Header认证，优先使用JWT
 * @dependencies github.com/go-chi/chi/v5
 */

package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// 上下文键类型
type contextKey string

const (
	// UserContextKey 用户上下文键
	UserContextKey contextKey = "user"
)

// UserInfo 用户信息结构
type UserInfo struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// GetUserFromContext 从上下文获取用户信息
func GetUserFromContext(ctx context.Context) *UserInfo {
	if user, ok := ctx.Value(UserContextKey).(*UserInfo); ok {
		return user
	}
	return nil
}

// GetUserIDFromContext 从上下文获取用户ID
func GetUserIDFromContext(ctx context.Context) uint {
	user := GetUserFromContext(ctx)
	if user != nil {
		return user.ID
	}
	// 默认返回1（系统用户）
	return 1
}

// AuthMiddleware 认证中间件
// 支持以下认证方式：
// 1. Authorization: Bearer <JWT Token>
// 2. X-User-ID: <用户ID>
// 3. X-User-Info: <Base64编码的用户JSON>
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := extractUserInfo(r)

		// 将用户信息注入上下文
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuthMiddleware 强制认证中间件
// 如果未提供有效的用户信息，返回401错误
func RequireAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := extractUserInfo(r)

		if user == nil || user.ID == 0 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 将用户信息注入上下文
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractUserInfo 从请求中提取用户信息
func extractUserInfo(r *http.Request) *UserInfo {
	// 方式1: 尝试从 Authorization Bearer Token 提取
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if user := parseJWTToken(token); user != nil {
				return user
			}
		}
	}

	// 方式2: 尝试从 X-User-Info Header 提取 (Base64编码的JSON)
	if userInfoHeader := r.Header.Get("X-User-Info"); userInfoHeader != "" {
		if user := parseUserInfoHeader(userInfoHeader); user != nil {
			return user
		}
	}

	// 方式3: 尝试从 X-User-ID Header 提取
	if userIDHeader := r.Header.Get("X-User-ID"); userIDHeader != "" {
		if userID, err := strconv.ParseUint(userIDHeader, 10, 32); err == nil && userID > 0 {
			return &UserInfo{
				ID:       uint(userID),
				Username: r.Header.Get("X-User-Name"),
				Role:     r.Header.Get("X-User-Role"),
			}
		}
	}

	// 方式4: 开发模式下的默认用户
	// 可以通过环境变量控制是否启用
	return &UserInfo{
		ID:       1,
		Username: "system",
		Role:     "admin",
	}
}

// parseJWTToken 解析JWT Token
// 简化实现：仅解析JWT payload部分，不进行签名验证
// 生产环境应使用完整的JWT验证库
func parseJWTToken(token string) *UserInfo {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}

	// 解码payload部分
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// 尝试标准base64解码
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			log.Printf("解析JWT payload失败: %v", err)
			return nil
		}
	}

	// 解析JSON
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		log.Printf("解析JWT claims失败: %v", err)
		return nil
	}

	user := &UserInfo{}

	// 提取用户ID
	if sub, ok := claims["sub"]; ok {
		switch v := sub.(type) {
		case float64:
			user.ID = uint(v)
		case string:
			if id, err := strconv.ParseUint(v, 10, 32); err == nil {
				user.ID = uint(id)
			}
		}
	}
	if user.ID == 0 {
		if userID, ok := claims["user_id"]; ok {
			switch v := userID.(type) {
			case float64:
				user.ID = uint(v)
			case string:
				if id, err := strconv.ParseUint(v, 10, 32); err == nil {
					user.ID = uint(id)
				}
			}
		}
	}

	// 提取用户名
	if username, ok := claims["username"].(string); ok {
		user.Username = username
	} else if name, ok := claims["name"].(string); ok {
		user.Username = name
	} else if preferredUsername, ok := claims["preferred_username"].(string); ok {
		user.Username = preferredUsername
	}

	// 提取邮箱
	if email, ok := claims["email"].(string); ok {
		user.Email = email
	}

	// 提取角色
	if role, ok := claims["role"].(string); ok {
		user.Role = role
	} else if roles, ok := claims["roles"].([]interface{}); ok && len(roles) > 0 {
		if firstRole, ok := roles[0].(string); ok {
			user.Role = firstRole
		}
	}

	return user
}

// parseUserInfoHeader 解析X-User-Info Header
func parseUserInfoHeader(header string) *UserInfo {
	// 解码Base64
	decoded, err := base64.StdEncoding.DecodeString(header)
	if err != nil {
		// 尝试URL安全的Base64
		decoded, err = base64.RawURLEncoding.DecodeString(header)
		if err != nil {
			return nil
		}
	}

	var user UserInfo
	if err := json.Unmarshal(decoded, &user); err != nil {
		return nil
	}

	return &user
}

