/*
 * @module service/llm_client
 * @description OpenAI 兼容大模型调用的公共实现，供 Python 脚本生成与 AI 快速编排共用。
 */

package service

import (
	"box-manage-service/config"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string                 `json:"model"`
	Temperature float64                `json:"temperature"`
	Messages    []chatMessage          `json:"messages"`
	ExtraBody   map[string]interface{} `json:"-"`
}

func (r chatCompletionRequest) MarshalJSON() ([]byte, error) {
	payload := map[string]interface{}{
		"model":       r.Model,
		"temperature": r.Temperature,
		"messages":    r.Messages,
	}
	for key, value := range r.ExtraBody {
		payload[key] = value
	}
	return json.Marshal(payload)
}

type chatCompletionResponse struct {
	Choices []struct {
		// FinishReason 结束原因：stop=正常结束，length=达到 max_tokens 被截断。
		// 截断时的内容是残缺的（JSON 通常括号不闭合），必须显式识别，
		// 否则会被当成「模型返回非法 JSON」反复空转修复轮次。
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// errLLMOutputTruncated 表示模型输出因长度上限被截断。
var errLLMOutputTruncated = errors.New("模型输出达到长度上限被截断")

// IsLLMOutputTruncated 判断错误是否为「输出被截断」。
// 上层据此区分「编排有问题」（可修复）与「配置上限太小」（重试无用）。
func IsLLMOutputTruncated(err error) bool {
	return errors.Is(err, errLLMOutputTruncated)
}

// llmClient 封装 OpenAI 兼容的 /chat/completions 调用。
// 不同的业务服务可以各自持有一个实例，共享同一份 config.LLMConfig。
type llmClient struct {
	cfg        config.LLMConfig
	httpClient *http.Client
}

// newLLMClient 创建 LLM 客户端；超时缺省为 60 秒。
func newLLMClient(cfg config.LLMConfig) *llmClient {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &llmClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// configError 检查必填配置，返回可直接展示给用户的错误。
func (c *llmClient) configError() error {
	if strings.TrimSpace(c.cfg.BaseURL) == "" {
		return errors.New("LLM API base URL is not configured")
	}
	if strings.TrimSpace(c.cfg.APIKey) == "" {
		return errors.New("LLM API key is not configured")
	}
	return nil
}

// maxRequestBytes 返回允许的提示词字节数上限，<=0 表示不限制。
func (c *llmClient) maxRequestBytes() int64 {
	return c.cfg.MaxRequestBytes
}

// model 返回当前配置的模型名。
func (c *llmClient) model() string {
	return c.cfg.Model
}

// complete 调用大模型并返回首条回复的文本内容。
func (c *llmClient) complete(ctx context.Context, messages []chatMessage) (string, error) {
	if err := c.configError(); err != nil {
		return "", err
	}
	extraBody := map[string]interface{}{
		"top_k": c.cfg.TopK,
		"chat_template_kwargs": map[string]bool{
			"enable_thinking": c.cfg.EnableThinking,
		},
	}
	// 显式指定输出上限，避免服务端默认值把长编排截断成残缺 JSON。
	// （此前不传该参数，修复轮次反复因截断而失败。）
	if c.cfg.MaxOutputTokens > 0 {
		extraBody["max_tokens"] = c.cfg.MaxOutputTokens
	}
	payload, err := json.Marshal(chatCompletionRequest{
		Model:       c.cfg.Model,
		Temperature: c.cfg.Temperature,
		Messages:    messages,
		ExtraBody:   extraBody,
	})
	if err != nil {
		return "", fmt.Errorf("marshal LLM request: %w", err)
	}
	requestCtx := ctx
	if c.cfg.RequestTimeout > 0 {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, c.cfg.RequestTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create LLM request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call LLM API: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read LLM response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("LLM API returned HTTP %d: %s", resp.StatusCode, truncateForLog(string(body), 500))
	}
	var completion chatCompletionResponse
	if err := json.Unmarshal(body, &completion); err != nil {
		return "", fmt.Errorf("decode LLM response: %w", err)
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return "", errors.New("LLM response contains no generated content")
	}
	// 输出被截断时内容是残缺的，直接判定失败并给出明确原因，
	// 避免上层把残缺 JSON 当成「模型不听话」继续重试。
	if completion.Choices[0].FinishReason == "length" {
		return "", fmt.Errorf("%w（max_tokens=%d），请调大 LLM_MAX_OUTPUT_TOKENS 或简化编排", errLLMOutputTruncated, c.cfg.MaxOutputTokens)
	}
	return completion.Choices[0].Message.Content, nil
}

// truncateForLog 截断日志文本，避免把整段响应写进日志。
func truncateForLog(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "...(truncated)"
}
