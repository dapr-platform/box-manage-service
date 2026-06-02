/*
 * @module service/executors/wechat_work_executor
 * @description 企业微信机器人消息推送节点执行器
 * @architecture 策略模式 - 企业微信 Webhook 执行器
 * @documentReference 企业微信机器人 Webhook 文档: https://developer.work.weixin.qq.com/document/path/91770
 * @stateFlow 组装URL → 模板变量替换 → 拼装body → POST Webhook → 返回结果
 * @rules key + msgtype 独立参数，url 自动拼接，body 支持 {var} 模板替换
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
	"regexp"
	"strings"
	"time"
)

const wechatWorkBaseURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send"

// WechatWorkExecutor 企业微信消息推送执行器
type WechatWorkExecutor struct {
	*BaseExecutor
	client *http.Client
}

// NewWechatWorkExecutor 创建企业微信执行器
func NewWechatWorkExecutor() *WechatWorkExecutor {
	return &WechatWorkExecutor{
		BaseExecutor: NewBaseExecutor("wechat_work"),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// WechatWorkConfig 企业微信推送配置
type WechatWorkConfig struct {
	Key     string                 `json:"key"`     // Webhook key
	MsgType string                 `json:"msgtype"` // text / markdown / markdown_v2 / news
	Body    map[string]interface{} `json:"body"`    // 对应 msgtype 的消息内容体
}

// Execute 执行企业微信消息推送
func (e *WechatWorkExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行企业微信推送节点"}

	config, err := e.parseConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	// 拼接 URL
	webhookURL := fmt.Sprintf("%s?key=%s", wechatWorkBaseURL, config.Key)
	logs = append(logs, fmt.Sprintf("消息类型: %s", config.MsgType))

	// 模板变量替换：递归替换 body 中所有字符串的 {key} → 变量值
	rendered := e.renderBody(config.Body, execCtx.Variables)

	// 拼装最终请求体: {"msgtype": "...", "<msgtype>": {...}}
	reqBody := map[string]interface{}{
		"msgtype":      config.MsgType,
		config.MsgType: rendered,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	logs = append(logs, fmt.Sprintf("请求体: %s", e.truncate(string(bodyBytes), 300)))

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		logs = append(logs, fmt.Sprintf("创建请求失败: %v", err))
		return CreateFailureResult(err, logs), err
	}
	req.Header.Set("Content-Type", "application/json")

	logs = append(logs, "发送企业微信消息...")
	startTime := time.Now()
	resp, err := e.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		logs = append(logs, fmt.Sprintf("请求失败: %v, 耗时: %v", err, duration))
		return CreateFailureResult(err, logs), err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var wxResp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	json.Unmarshal(respBody, &wxResp)

	extras := map[string]interface{}{
		"errcode":     wxResp.ErrCode,
		"errmsg":      wxResp.ErrMsg,
		"status_code": resp.StatusCode,
		"duration_ms": duration.Milliseconds(),
	}
	outputs := CreateOutputs(map[string]interface{}{"result": wxResp.ErrMsg}, extras)

	if wxResp.ErrCode != 0 {
		errMsg := fmt.Sprintf("企业微信返回错误: errcode=%d, errmsg=%s", wxResp.ErrCode, wxResp.ErrMsg)
		logs = append(logs, errMsg)
		return CreateFailureResult(fmt.Errorf(errMsg), logs), fmt.Errorf(errMsg)
	}

	logs = append(logs, fmt.Sprintf("企业微信推送成功, 耗时: %v", duration))
	return CreateSuccessResult(outputs, logs), nil
}

// Validate 验证节点配置
func (e *WechatWorkExecutor) Validate(nodeInstance *models.NodeInstance) error {
	if err := e.BaseExecutor.Validate(nodeInstance); err != nil {
		return err
	}
	return nil
}

// parseConfig 解析输入配置
func (e *WechatWorkExecutor) parseConfig(inputs map[string]interface{}) (*WechatWorkConfig, error) {
	config := &WechatWorkConfig{
		MsgType: "text",
	}

	if key, ok := inputs["key"].(string); ok && key != "" {
		config.Key = key
	} else {
		return nil, fmt.Errorf("缺少必需参数: key")
	}

	if msgType, ok := inputs["msgtype"].(string); ok && msgType != "" {
		config.MsgType = msgType
	}

	if body, ok := inputs["body"]; ok {
		if bodyMap, ok := body.(map[string]interface{}); ok {
			config.Body = bodyMap
		}
	}

	if config.Body == nil {
		// body 为空时给默认空内容
		config.Body = map[string]interface{}{"content": ""}
	}

	return config, nil
}

// renderBody 递归替换所有字符串值中的 {key} 模板变量
func (e *WechatWorkExecutor) renderBody(body map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(body))
	for k, v := range body {
		result[k] = e.renderValue(v, variables)
	}
	return result
}

func (e *WechatWorkExecutor) renderValue(v interface{}, variables map[string]interface{}) interface{} {
	switch val := v.(type) {
	case string:
		return e.renderTemplate(val, variables)
	case map[string]interface{}:
		return e.renderBody(val, variables)
	case []interface{}:
		arr := make([]interface{}, len(val))
		for i, item := range val {
			arr[i] = e.renderValue(item, variables)
		}
		return arr
	default:
		return v
	}
}

var templateRe = regexp.MustCompile(`\{([a-zA-Z_]\w*(?:\.[a-zA-Z_]\w*)*)\}`)

// renderTemplate 将 {key} 或 {parent.child} 替换为变量值
func (e *WechatWorkExecutor) renderTemplate(template string, variables map[string]interface{}) string {
	if template == "" || variables == nil {
		return template
	}
	return templateRe.ReplaceAllStringFunc(template, func(match string) string {
		key := match[1 : len(match)-1]
		if val, ok := e.resolvePath(key, variables); ok {
			return fmt.Sprintf("%v", val)
		}
		return match
	})
}

func (e *WechatWorkExecutor) resolvePath(path string, variables map[string]interface{}) (interface{}, bool) {
	parts := strings.Split(path, ".")
	current := variables
	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			// 尝试从任意 map 类型变量值中查找
			for _, v := range variables {
				if m, ok := v.(map[string]interface{}); ok {
					if found, ok2 := e.resolvePathDeep(path, m); ok2 {
						return found, true
					}
				}
			}
			return nil, false
		}
		if i == len(parts)-1 {
			return val, true
		}
		if nextMap, ok := val.(map[string]interface{}); ok {
			current = nextMap
		} else {
			return nil, false
		}
	}
	return nil, false
}

func (e *WechatWorkExecutor) resolvePathDeep(path string, m map[string]interface{}) (interface{}, bool) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, false
	}
	val, ok := m[parts[0]]
	if !ok {
		return nil, false
	}
	if len(parts) == 1 {
		return val, true
	}
	if sub, ok := val.(map[string]interface{}); ok {
		rest := strings.Join(parts[1:], ".")
		return e.resolvePathDeep(rest, sub)
	}
	return nil, false
}

func (e *WechatWorkExecutor) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
