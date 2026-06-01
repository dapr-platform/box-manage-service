/*
 * @module service/executors/wechat_work_executor
 * @description 企业微信机器人消息推送节点执行器
 * @architecture 策略模式 - 企业微信 Webhook 执行器
 * @documentReference 企业微信机器人 Webhook 文档: https://developer.work.weixin.qq.com/document/path/91770
 * @stateFlow 解析配置 → 替换模板变量 → POST Webhook → 返回结果
 * @rules 支持 text / markdown / news 三种消息类型，content 支持 {{key}} 模板变量替换
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
	WebhookURL    string `json:"webhook_url"`    // Webhook 地址
	MsgType       string `json:"msgtype"`        // text / markdown / news
	Content       string `json:"content"`        // 消息内容，支持 {{变量名}} 模板
	MentionedList string `json:"mentioned_list"` // @的用户列表，逗号分隔（仅 text 类型有效）
}

// Execute 执行企业微信消息推送
func (e *WechatWorkExecutor) Execute(ctx context.Context, execCtx *ExecutionContext) (*ExecutionResult, error) {
	logs := []string{"开始执行企业微信推送节点"}

	config, err := e.parseConfig(execCtx.Inputs)
	if err != nil {
		logs = append(logs, fmt.Sprintf("解析配置失败: %v", err))
		return CreateFailureResult(err, logs), err
	}

	// 模板变量替换：将 {{key}} 替换为变量值
	content := e.renderTemplate(config.Content, execCtx.Variables)
	logs = append(logs, fmt.Sprintf("消息类型: %s, 内容: %s", config.MsgType, e.truncate(content, 100)))

	// 构建请求体
	body := map[string]interface{}{
		"msgtype": config.MsgType,
	}

	switch config.MsgType {
	case "text":
		textBody := map[string]interface{}{"content": content}
		if config.MentionedList != "" {
			mentioned := strings.Split(config.MentionedList, ",")
			var trimmed []string
			for _, m := range mentioned {
				if t := strings.TrimSpace(m); t != "" {
					trimmed = append(trimmed, t)
				}
			}
			textBody["mentioned_list"] = trimmed
		}
		body["text"] = textBody

	case "markdown":
		body["markdown"] = map[string]interface{}{"content": content}

	case "news":
		// news 类型 content 应为 JSON 数组字符串: [{"title":"...","description":"...","url":"...","picurl":"..."}]
		var articles []interface{}
		if err := json.Unmarshal([]byte(content), &articles); err != nil {
			logs = append(logs, fmt.Sprintf("news 内容解析失败，应为 JSON 数组: %v", err))
			return CreateFailureResult(err, logs), err
		}
		body["news"] = map[string]interface{}{"articles": articles}

	default:
		err := fmt.Errorf("不支持的消息类型: %s（支持 text / markdown / news）", config.MsgType)
		logs = append(logs, err.Error())
		return CreateFailureResult(err, logs), err
	}

	// 序列化请求
	bodyBytes, _ := json.Marshal(body)
	logs = append(logs, fmt.Sprintf("请求体: %s", e.truncate(string(bodyBytes), 200)))

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "POST", config.WebhookURL, bytes.NewBuffer(bodyBytes))
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

	// 企业微信返回格式: {"errcode":0,"errmsg":"ok"}
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

	if url, ok := inputs["webhook_url"].(string); ok && url != "" {
		config.WebhookURL = url
	} else {
		return nil, fmt.Errorf("缺少必需参数: webhook_url")
	}

	if msgType, ok := inputs["msgtype"].(string); ok && msgType != "" {
		config.MsgType = msgType
	}

	if content, ok := inputs["content"].(string); ok {
		config.Content = content
	}

	if mentioned, ok := inputs["mentioned_list"].(string); ok {
		config.MentionedList = mentioned
	}

	return config, nil
}

// renderTemplate 将 {{key}} 替换为变量值
func (e *WechatWorkExecutor) renderTemplate(template string, variables map[string]interface{}) string {
	if template == "" || variables == nil {
		return template
	}
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	return re.ReplaceAllStringFunc(template, func(match string) string {
		key := match[2 : len(match)-2]
		if val, ok := variables[key]; ok {
			return fmt.Sprintf("%v", val)
		}
		return match // 未匹配的保持原样
	})
}

func (e *WechatWorkExecutor) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
