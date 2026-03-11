package middleware

import (
	"crypto/subtle"
	"net/http"
)

// SwaggerBasicAuth 为 Swagger 提供基础认证中间件
func SwaggerBasicAuth(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果没有设置密码，则不进行认证
			if password == "" {
				next.ServeHTTP(w, r)
				return
			}

			// 获取基础认证信息
			user, pass, ok := r.BasicAuth()

			// 使用常量时间比较防止时序攻击
			if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 ||
				subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="Swagger Documentation"`)
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("Unauthorized: Invalid credentials"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
