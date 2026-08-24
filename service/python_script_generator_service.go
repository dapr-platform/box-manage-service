/*
 * @module service/python_script_generator_service
 * @description 使用 OpenAI 兼容大模型生成 DAG Python 节点脚本
 */

package service

import (
	"box-manage-service/config"
	"box-manage-service/models"
	"box-manage-service/repository"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

const pythonScriptSystemPrompt = `你是 DAG 工作流中的 Python 脚本生成助手。
硬性规则：
1. 必须定义入口 def run(ctx) -> dict，入口名称和参数不可修改。
2. 当前节点的输入参数定义在 node_inputs 中（若调用方使用兼容字段，则读取 inputs）：结合 upstream 和 mapping 从 ctx.inputs 读取，并使用参数的 key_name 作为字段契约。
3. 当前节点的输出参数定义在 node_outputs 中（若调用方使用兼容字段，则读取 outputs）：返回值必须是 dict，且字段名、类型和必填约束要符合 node_outputs；output_schema 是补充约束。
4. 当前节点配置定义在 node_config 中（若调用方使用兼容字段，则读取 config），运行时通过 ctx.config 读取；不要把配置中的可配置值硬编码进脚本。
5. 通过 ctx.vars 读取工作流变量。只能使用标准库 json、math、datetime、typing；禁止文件读写、网络、子进程、eval、exec、compile 和动态导入。
6. 只输出完整 Python 源码，不要 markdown 代码围栏，不要解释文字。
请先理解当前节点输入/输出参数、节点配置、上游契约和样例，再生成最小、可读、可执行的实现。`

// PythonScriptUpstream 描述一个上游节点及其数据契约。
type PythonScriptUpstream struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name,omitempty"`
	Schema map[string]interface{} `json:"schema,omitempty"`
	Sample interface{}            `json:"sample,omitempty"`
}

// PythonScriptNodeParameter 描述当前 Python 节点的输入或输出参数。
// KeyName 应与工作流变量定义中的 key_name 一致；其余字段用于帮助模型理解契约。
type PythonScriptNodeParameter struct {
	KeyName      string      `json:"key_name"`
	Name         string      `json:"name,omitempty"`
	Type         string      `json:"type,omitempty"`
	Direction    string      `json:"direction,omitempty"`
	Required     bool        `json:"required,omitempty"`
	DefaultValue interface{} `json:"default_value,omitempty"`
	RefKeyName   string      `json:"ref_key_name,omitempty"`
	Description  string      `json:"description,omitempty"`
	Schema       interface{} `json:"schema,omitempty"`
}

// PythonScriptGenerateRequest 是前端提交的脚本生成上下文。
type PythonScriptGenerateRequest struct {
	Description string                      `json:"description"`
	WorkflowID  uint                        `json:"workflow_id,omitempty"`
	NodeID      string                      `json:"node_id,omitempty"`
	Upstream    []PythonScriptUpstream      `json:"upstream,omitempty"`
	NodeInputs  []PythonScriptNodeParameter `json:"node_inputs,omitempty"`
	NodeOutputs []PythonScriptNodeParameter `json:"node_outputs,omitempty"`
	NodeConfig  map[string]interface{}      `json:"node_config,omitempty"`
	// Inputs/Outputs 兼容直接传入 NodeDefinition.inputs/outputs 的调用方。
	Inputs       interface{}            `json:"inputs,omitempty"`
	Outputs      interface{}            `json:"outputs,omitempty"`
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

type PythonScriptNodeContext struct {
	Inputs  []PythonScriptNodeParameter
	Outputs []PythonScriptNodeParameter
	Config  map[string]interface{}
}

type pythonScriptNodeContextResolver interface {
	Resolve(ctx context.Context, workflowID uint, nodeID string) (*PythonScriptNodeContext, error)
}

type pythonScriptGeneratorService struct {
	cfg        config.LLMConfig
	httpClient *http.Client
	resolver   pythonScriptNodeContextResolver
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

// NewPythonScriptGeneratorServiceWithDB 创建带数据库上下文解析能力的脚本生成服务。
// 生成请求只需提供 workflow_id 和 node_id，节点参数与配置由后端查询填充。
func NewPythonScriptGeneratorServiceWithDB(cfg config.LLMConfig, db *gorm.DB) PythonScriptGeneratorService {
	service := NewPythonScriptGeneratorService(cfg).(*pythonScriptGeneratorService)
	if db != nil {
		service.resolver = &pythonScriptNodeContextResolverImpl{
			nodeRepo:     repository.NewNodeDefinitionRepository(db),
			variableRepo: repository.NewVariableDefinitionRepository(db),
		}
	}
	return service
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
	if req.WorkflowID > 0 || strings.TrimSpace(req.NodeID) != "" {
		if req.WorkflowID == 0 || strings.TrimSpace(req.NodeID) == "" {
			return nil, errors.New("workflow_id and node_id are required together")
		}
		if s.resolver == nil {
			return nil, errors.New("node context resolver is not configured")
		}
		nodeContext, err := s.resolver.Resolve(ctx, req.WorkflowID, req.NodeID)
		if err != nil {
			return nil, err
		}
		// 后端查询结果覆盖请求中的同名字段，避免前端篡改节点契约。
		req.NodeInputs = nodeContext.Inputs
		req.NodeOutputs = nodeContext.Outputs
		req.NodeConfig = nodeContext.Config
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

type pythonScriptNodeContextResolverImpl struct {
	nodeRepo     repository.NodeDefinitionRepository
	variableRepo repository.VariableDefinitionRepository
}

func (r *pythonScriptNodeContextResolverImpl) Resolve(ctx context.Context, workflowID uint, nodeID string) (*PythonScriptNodeContext, error) {
	nodes, err := r.nodeRepo.FindByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("query node definitions: %w", err)
	}
	var node *models.NodeDefinition
	for _, candidate := range nodes {
		if candidate.NodeID == nodeID || candidate.NodeKeyName == nodeID {
			node = candidate
			break
		}
	}
	if node == nil {
		return nil, fmt.Errorf("node %q not found in workflow %d", nodeID, workflowID)
	}

	result := &PythonScriptNodeContext{
		Inputs:  make([]PythonScriptNodeParameter, 0),
		Outputs: make([]PythonScriptNodeParameter, 0),
		Config:  map[string]interface{}(node.Config),
	}
	variables, err := r.variableRepo.FindByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("query variable definitions: %w", err)
	}
	for _, variable := range variables {
		if variable.NodeID != node.NodeID {
			continue
		}
		parameter := PythonScriptNodeParameter{
			KeyName:      variable.KeyName,
			Name:         variable.Name,
			Type:         variable.Type,
			Direction:    string(variable.Direction),
			Required:     variable.Required,
			DefaultValue: variable.DefaultValue,
			RefKeyName:   variable.RefKeyName,
			Description:  variable.Description,
		}
		if variable.Direction == models.VariableDirectionOutput {
			result.Outputs = append(result.Outputs, parameter)
		} else {
			result.Inputs = append(result.Inputs, parameter)
		}
	}
	return result, nil
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
