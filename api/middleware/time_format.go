/*
 * @module api/middleware/time_format
 * @description 时间格式统一中间件 - 将响应中的时间格式统一为 yyyy-mm-dd hh:mm:ss
 * @architecture 中间件层
 * @documentReference REQ-TIME: 时间格式统一
 * @stateFlow HTTP响应 -> 时间格式转换 -> 返回客户端
 * @rules 将所有 RFC3339 格式的时间转换为 yyyy-mm-dd hh:mm:ss 格式
 * @dependencies github.com/go-chi/chi/v5
 */

package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"time"
)

// timeFormatResponseWriter 包装 ResponseWriter 以转换时间格式
type timeFormatResponseWriter struct {
	http.ResponseWriter
	body *bytes.Buffer
}

// newTimeFormatResponseWriter 创建新的 timeFormatResponseWriter
func newTimeFormatResponseWriter(w http.ResponseWriter) *timeFormatResponseWriter {
	return &timeFormatResponseWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
	}
}

// Write 捕获响应体
func (w *timeFormatResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

// WriteHeader 保持原有的状态码写入
func (w *timeFormatResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
}

// TimeFormatMiddleware 时间格式统一中间件
// 将响应中的所有时间字段从 RFC3339 格式转换为 yyyy-mm-dd hh:mm:ss 格式
func TimeFormatMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 包装 ResponseWriter
		wrapped := newTimeFormatResponseWriter(w)

		// 调用下一个处理器
		next.ServeHTTP(wrapped, r)

		// 获取响应体
		responseBody := wrapped.body.Bytes()

		// 只处理 JSON 响应
		contentType := wrapped.Header().Get("Content-Type")
		if len(responseBody) > 0 && (contentType == "" || regexp.MustCompile(`application/json`).MatchString(contentType)) {
			// 转换时间格式
			convertedBody := convertTimeFormat(responseBody)

			// 更新 Content-Length
			wrapped.ResponseWriter.Header().Set("Content-Length", string(rune(len(convertedBody))))

			// 写入转换后的响应
			wrapped.ResponseWriter.Write(convertedBody)
		} else {
			// 非 JSON 响应，直接写入
			wrapped.ResponseWriter.Write(responseBody)
		}
	})
}

// convertTimeFormat 转换响应体中的时间格式
func convertTimeFormat(body []byte) []byte {
	// RFC3339 时间格式的正则表达式
	// 匹配格式：2006-01-02T15:04:05Z 或 2006-01-02T15:04:05+08:00
	rfc3339Pattern := regexp.MustCompile(`"(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})"`)

	// 替换为 yyyy-mm-dd hh:mm:ss 格式
	result := rfc3339Pattern.ReplaceAllFunc(body, func(match []byte) []byte {
		// 提取时间字符串（去掉引号）
		timeStr := string(match[1 : len(match)-1])

		// 尝试解析时间
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05+08:00",
		}

		var t time.Time
		var err error
		for _, format := range formats {
			t, err = time.Parse(format, timeStr)
			if err == nil {
				break
			}
		}

		if err != nil {
			// 解析失败，返回原值
			return match
		}

		// 转换为目标格式
		formatted := t.Format("2006-01-02 15:04:05")
		return []byte(`"` + formatted + `"`)
	})

	return result
}

// ConvertTimeInJSON 转换 JSON 对象中的时间字段（递归处理）
func ConvertTimeInJSON(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			v[key] = ConvertTimeInJSON(value)
		}
		return v
	case []interface{}:
		for i, value := range v {
			v[i] = ConvertTimeInJSON(value)
		}
		return v
	case string:
		// 尝试解析为时间
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return t.Format("2006-01-02 15:04:05")
		}
		return v
	default:
		return v
	}
}

// ConvertJSONTimeFormat 转换 JSON 字节数组中的时间格式（深度转换）
func ConvertJSONTimeFormat(jsonData []byte) ([]byte, error) {
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return jsonData, err
	}

	converted := ConvertTimeInJSON(data)

	return json.Marshal(converted)
}
