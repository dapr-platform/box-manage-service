/*
 * @module service/python_script_generator_service
 * @description 使用 OpenAI 兼容大模型生成 DAG Python 节点脚本
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

const pythonScriptSystemPrompt = `你是 DAG 工作流中的 Python 脚本生成助手。
硬性规则：
1. 必须定义入口 def run(ctx) -> dict，入口名称和参数不可修改。
2. 通过 ctx.inputs[上游节点 id] 读取输入，通过 ctx.vars 和 ctx.config 读取变量与静态配置。
3. 只能使用标准库 json、math、datetime、typing；禁止文件读写、网络、子进程、eval、exec、compile 和动态导入。
4. 返回值必须符合给定的 output schema。
5. 只输出完整 Python 源码，不要 markdown 代码围栏，不要解释文字。
请先理解输入输出契约和样例，再生成最小、可读、可执行的实现。`

// PythonScriptUpstream 描述一个上游节点及其数据契约。
type PythonScriptUpstream struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name,omitempty"`
	Schema map[string]interface{} `json:"schema,omitempty"`
	Sample interface{}            `json:"sample,omitempty"`
}

// PythonScriptGenerateRequest 是前端提交的脚本生成上下文。
type PythonScriptGenerateRequest struct {
	Description  string                 `json:"description"`
	Upstream     []PythonScriptUpstream `json:"upstream,omitempty"`
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"`
	Mapping      interface{}            `json:"mapping,omitempty"`
	VarsSchema   map[string]interface{} `json:"vars_schema,omitempty"`
	Vars         map[string]interface{} `json:"vars,omitempty"`
	ConfigSchema map[string]interface{} `json:"config_schema,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Examples     []interface{}          `json:"examples,omitempty"`
}

// PythonScriptDraft 是未落库的脚本草稿，供前端预览和编辑。
type PythonScriptDraft struct {
	Code         string   `json:"code"`
	LogicSummary string   `json:"logic_summary,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	UsedInputs   []string `json:"used_inputs,omitempty"`
	Model        string   `json:"model,omitempty"`
}

// PythonScriptGeneratorService 只负责生成草稿，不执行或持久化脚本。
type PythonScriptGeneratorService interface {
	Generate(ctx context.Context, req *PythonScriptGenerateRequest) (*PythonScriptDraft, error)
}

type pythonScriptGeneratorService struct {
	cfg        config.LLMConfig
	httpClient *http.Client
}

// NewPythonScriptGeneratorService 创建 Python 脚本生成服务。
func NewPythonScriptGeneratorService(cfg config.LLMConfig) PythonScriptGeneratorService {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &pythonScriptGeneratorService{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (s *pythonScriptGeneratorService) Generate(ctx context.Context, req *PythonScriptGenerateRequest) (*PythonScriptDraft, error) {
	if req == nil || strings.TrimSpace(req.Description) == "" {
		return nil, errors.New("description is required")
	}
	if strings.TrimSpace(s.cfg.BaseURL) == "" {
		return nil, errors.New("LLM API base URL is not configured")
	}
	if strings.TrimSpace(s.cfg.APIKey) == "" {
		return nil, errors.New("LLM API key is not configured")
	}

	userPrompt, err := buildPythonScriptUserPrompt(req)
	if err != nil {
		return nil, err
	}
	if s.cfg.MaxRequestBytes > 0 && int64(len(userPrompt)) > s.cfg.MaxRequestBytes {
		return nil, fmt.Errorf("generation context exceeds %d bytes", s.cfg.MaxRequestBytes)
	}

	messages := []chatMessage{
		{Role: "system", Content: pythonScriptSystemPrompt},
		{Role: "user", Content: userPrompt},
	}
	raw, err := s.complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	code := extractPythonCode(raw)
	if err := validatePythonScript(code, s.cfg.MaxCodeBytes); err != nil {
		// 生成结果不满足固定入口时，回灌一次确定性错误，便于模型自动修复。
		repairMessages := append(append([]chatMessage{}, messages...),
			chatMessage{Role: "assistant", Content: code},
			chatMessage{Role: "user", Content: "上一次生成未通过校验：" + err.Error() + "。请修正并只返回完整 Python 源码。"},
		)
		raw, repairErr := s.complete(ctx, repairMessages)
		if repairErr != nil {
			return nil, fmt.Errorf("generated script validation failed: %v; repair failed: %w", err, repairErr)
		}
		code = extractPythonCode(raw)
		if repairErr = validatePythonScript(code, s.cfg.MaxCodeBytes); repairErr != nil {
			return nil, fmt.Errorf("generated script validation failed after repair: %w", repairErr)
		}
	}

	return &PythonScriptDraft{
		Code:       code,
		Warnings:   []string{"脚本仅完成生成和入口校验，未在服务端执行；应用前请在沙箱中试跑。"},
		UsedInputs: upstreamIDs(req.Upstream),
		Model:      s.cfg.Model,
	}, nil
}

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
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (s *pythonScriptGeneratorService) complete(ctx context.Context, messages []chatMessage) (string, error) {
	payload, err := json.Marshal(chatCompletionRequest{
		Model:       s.cfg.Model,
		Temperature: s.cfg.Temperature,
		Messages:    messages,
		ExtraBody: map[string]interface{}{
			"top_k": s.cfg.TopK,
			"chat_template_kwargs": map[string]bool{
				"enable_thinking": s.cfg.EnableThinking,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal LLM request: %w", err)
	}
	requestCtx := ctx
	if s.cfg.RequestTimeout > 0 {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, s.cfg.RequestTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, strings.TrimRight(s.cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create LLM request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call LLM API: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read LLM response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("LLM API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var completion chatCompletionResponse
	if err := json.Unmarshal(body, &completion); err != nil {
		return "", fmt.Errorf("decode LLM response: %w", err)
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return "", errors.New("LLM response contains no generated content")
	}
	return completion.Choices[0].Message.Content, nil
}

func buildPythonScriptUserPrompt(req *PythonScriptGenerateRequest) (string, error) {
	contextJSON, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal generation context: %w", err)
	}
	return "请根据以下结构化 DAG 节点上下文生成 Python 脚本。只返回源码。\n\n" + string(contextJSON), nil
}

func extractPythonCode(raw string) string {
	code := strings.TrimSpace(raw)
	if start := strings.Index(code, "```"); start >= 0 {
		bodyStart := strings.IndexByte(code[start+3:], '\n')
		if bodyStart >= 0 {
			bodyStart += start + 3 + 1
			if end := strings.Index(code[bodyStart:], "```"); end >= 0 {
				return strings.TrimSpace(code[bodyStart : bodyStart+end])
			}
		}
	}
	return code
}

func validatePythonScript(code string, maxBytes int) error {
	if strings.TrimSpace(code) == "" {
		return errors.New("generated code is empty")
	}
	if maxBytes > 0 && len(code) > maxBytes {
		return fmt.Errorf("generated code exceeds %d bytes", maxBytes)
	}
	if !strings.Contains(code, "def run(ctx") {
		return errors.New("generated code must define def run(ctx)")
	}
	for _, forbidden := range []string{"eval(", "exec(", "compile(", "__import__(", "import os", "import subprocess", "import requests", "import httpx"} {
		if strings.Contains(code, forbidden) {
			return fmt.Errorf("generated code contains forbidden operation %q", forbidden)
		}
	}
	return nil
}

func upstreamIDs(upstream []PythonScriptUpstream) []string {
	ids := make([]string, 0, len(upstream))
	for _, node := range upstream {
		if id := strings.TrimSpace(node.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
