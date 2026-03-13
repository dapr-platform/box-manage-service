/*
 * @module service/executors/http_request_executor
 * @description HTTP请求节点执行器
 * @architecture 策略模式 - HTTP请求执行器实现
 * @documentReference 业务编排引擎设计文档
 * @stateFlow 发起HTTP请求 -> 等待响应 -> 返回结果
 * @rules 支持GET/POST/PUT/DELETE等HTTP方法，支持请求头、请求体、超时配置
 * @dependencies net/http, models
 */

package executors

import (
	"box-manage-service/models"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPRequestExecutor HTTP请求执行器
type HTTPRequestExecutor struct {
	*BaseExecutor
	client *http.Client
}

// NewHTTPRequestExecutor 创建HTTP请求执行器
func NewHTTPRequestExecutor() *HTTPRequestExecutor {
	return &HTTPRequestExecutor{
		BaseExecutor: NewBaseExecutor("http_request"),
		client: &http.Client{
			Timeout: 30 * time.Second, // 默认30秒超时
		},
	}
}

// Execute 执行HTTP请求
func (e *HTTPRequestExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行HTTP请求节点"}

	// 解析配置
	config, err := e.parseConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	logs = append(logs, fmt.Sprintf("请求方法: %s, URL: %s", config.Method, config.URL))

	// 设置超时
	if config.Timeout > 0 {
		e.client.Timeout = time.Duration(config.Timeout) * time.Second
	}

	// 创建请求
	var reqBody io.Reader
	if config.Body != nil {
		bodyBytes, err := json.Marshal(config.Body)
		if err != nil {
			logs = append(logs, fmt.Sprintf("序列化请求体失败: %v", err))
			return CreateFailureResult(err, logs), err
		}
		reqBody = bytes.NewBuffer(bodyBytes)
		logs = append(logs, fmt.Sprintf("请求体: %s", string(bodyBytes)))
	}

	req, err := http.NewRequestWithContext(ctx, config.Method, config.URL, reqBody)
	if err != nil {
		logs = append(logs, fmt.Sprintf("创建请求失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	// 设置请求头
	for key, value := range config.Headers {
		req.Header.Set(key, value)
	}
	if config.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 发送请求
	logs = append(logs, "发送HTTP请求...")
	startTime := time.Now()
	resp, err := e.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		logs = append(logs, fmt.Sprintf("请求失败: %v, 耗时: %v", err, duration))
		return CreateFailureResult(err, logs), err
	}
	defer resp.Body.Close()

	logs = append(logs, fmt.Sprintf("收到响应: 状态码=%d, 耗时=%v", resp.StatusCode, duration))

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logs = append(logs, fmt.Sprintf("读取响应体失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	// 解析响应
	var responseData interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &responseData); err != nil {
			// 如果不是JSON，返回原始字符串
			responseData = string(respBody)
		}
	}

	// 构建输出
	outputs := map[string]interface{}{
		"status_code":      resp.StatusCode,
		"response_body":    responseData,
		"response_headers": resp.Header,
		"duration_ms":      duration.Milliseconds(),
	}

	// 检查状态码
	if resp.StatusCode >= 400 {
		errMsg := fmt.Sprintf("HTTP请求失败: 状态码=%d", resp.StatusCode)
		logs = append(logs, errMsg)
		return CreateFailureResult(fmt.Errorf(errMsg), logs), fmt.Errorf(errMsg)
	}

	logs = append(logs, "HTTP请求执行成功")
	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证节点配置
func (e *HTTPRequestExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	// 可以添加更多验证逻辑
	return nil
}

// HTTPRequestConfig HTTP请求配置
type HTTPRequestConfig struct {
	Method  string            `json:"method"`  // GET/POST/PUT/DELETE等
	URL     string            `json:"url"`     // 请求URL
	Headers map[string]string `json:"headers"` // 请求头
	Body    interface{}       `json:"body"`    // 请求体
	Timeout int               `json:"timeout"` // 超时时间（秒）
}

// parseConfig 解析配置
func (e *HTTPRequestExecutor) parseConfig(inputs map[string]interface{}) (*HTTPRequestConfig, error) {
	config := &HTTPRequestConfig{
		Method:  "GET",
		Headers: make(map[string]string),
		Timeout: 30,
	}

	if method, ok := inputs["method"].(string); ok {
		config.Method = method
	}

	if url, ok := inputs["url"].(string); ok {
		config.URL = url
	} else {
		return nil, fmt.Errorf("缺少必需参数: url")
	}

	if headers, ok := inputs["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if strVal, ok := v.(string); ok {
				config.Headers[k] = strVal
			}
		}
	}

	if body, ok := inputs["body"]; ok {
		config.Body = body
	}

	if timeout, ok := inputs["timeout"].(float64); ok {
		config.Timeout = int(timeout)
	}

	return config, nil
}
