/*
 * @module api/middleware/logger
 * @description HTTP请求日志中间件 - 记录请求参数和响应
 * @architecture 中间件层
 * @documentReference REQ-LOG: 请求日志记录
 * @stateFlow HTTP请求 -> 记录请求 -> 处理 -> 记录响应
 * @rules 记录请求方法、路径、参数、响应状态码、响应体和耗时
 * @dependencies github.com/go-chi/chi/v5
 */

package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// responseWriter 包装http.ResponseWriter以捕获响应数据
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

// newResponseWriter 创建新的responseWriter
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
	}
}

// WriteHeader 捕获状态码
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write 捕获响应体
func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// LoggerMiddleware HTTP请求日志中间件
// 记录请求的详细信息和响应数据
func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// 读取请求体
		var requestBody []byte
		if r.Body != nil {
			requestBody, _ = io.ReadAll(r.Body)
			// 重新设置请求体，以便后续处理器可以读取
			r.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// 包装ResponseWriter以捕获响应
		wrappedWriter := newResponseWriter(w)

		// 记录请求信息
		logRequest(r, requestBody)

		// 处理请求
		next.ServeHTTP(wrappedWriter, r)

		// 计算耗时
		duration := time.Since(startTime)

		// 记录响应信息
		logResponse(r, wrappedWriter, duration)
	})
}

// logRequest 记录请求信息
func logRequest(r *http.Request, body []byte) {
	var logParts []string

	// 基本信息
	logParts = append(logParts, "========== HTTP Request ==========")
	logParts = append(logParts, "Method: "+r.Method)
	logParts = append(logParts, "URL: "+r.URL.String())
	logParts = append(logParts, "Path: "+r.URL.Path)

	// 查询参数
	if len(r.URL.Query()) > 0 {
		queryParams, _ := json.MarshalIndent(r.URL.Query(), "", "  ")
		logParts = append(logParts, "Query Params: "+string(queryParams))
	}

	// 请求头（过滤敏感信息）
	headers := make(map[string]string)
	for key, values := range r.Header {
		// 过滤敏感头信息
		if strings.ToLower(key) == "authorization" {
			headers[key] = "[FILTERED]"
		} else if strings.ToLower(key) == "cookie" {
			headers[key] = "[FILTERED]"
		} else {
			headers[key] = strings.Join(values, ", ")
		}
	}
	if len(headers) > 0 {
		headersJSON, _ := json.MarshalIndent(headers, "", "  ")
		logParts = append(logParts, "Headers: "+string(headersJSON))
	}

	// 请求体
	if len(body) > 0 {
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "application/json") {
			// 格式化JSON
			var jsonData interface{}
			if err := json.Unmarshal(body, &jsonData); err == nil {
				prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
				logParts = append(logParts, "Request Body: "+string(prettyJSON))
			} else {
				logParts = append(logParts, "Request Body: "+string(body))
			}
		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			logParts = append(logParts, "Request Body (Form): "+string(body))
		} else if strings.Contains(contentType, "multipart/form-data") {
			logParts = append(logParts, "Request Body: [Multipart Form Data]")
		} else {
			// 限制非JSON数据的长度
			if len(body) > 1000 {
				logParts = append(logParts, "Request Body: "+string(body[:1000])+"... [truncated]")
			} else {
				logParts = append(logParts, "Request Body: "+string(body))
			}
		}
	}

	// 客户端信息
	logParts = append(logParts, "Remote Addr: "+r.RemoteAddr)
	if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
		logParts = append(logParts, "User-Agent: "+userAgent)
	}

	log.Println(strings.Join(logParts, "\n"))
}

// logResponse 记录响应信息
func logResponse(r *http.Request, rw *responseWriter, duration time.Duration) {
	var logParts []string

	logParts = append(logParts, "========== HTTP Response ==========")
	logParts = append(logParts, "Method: "+r.Method)
	logParts = append(logParts, "Path: "+r.URL.Path)
	logParts = append(logParts, "Status: "+http.StatusText(rw.statusCode)+" ("+string(rune(rw.statusCode))+")")
	logParts = append(logParts, "Duration: "+duration.String())

	// 响应体
	if rw.body.Len() > 0 {
		responseBody := rw.body.Bytes()
		contentType := rw.Header().Get("Content-Type")

		if strings.Contains(contentType, "application/json") {
			// 格式化JSON响应
			var jsonData interface{}
			if err := json.Unmarshal(responseBody, &jsonData); err == nil {
				prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
				logParts = append(logParts, "Response Body: "+string(prettyJSON))
			} else {
				logParts = append(logParts, "Response Body: "+string(responseBody))
			}
		} else {
			// 限制非JSON响应的长度
			if len(responseBody) > 1000 {
				logParts = append(logParts, "Response Body: "+string(responseBody[:1000])+"... [truncated]")
			} else {
				logParts = append(logParts, "Response Body: "+string(responseBody))
			}
		}
	}

	logParts = append(logParts, "==================================")

	log.Println(strings.Join(logParts, "\n"))
}
