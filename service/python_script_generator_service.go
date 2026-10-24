/*
 * @module service/python_script_generator_service
 * @description 使用 OpenAI 兼容大模型生成 DAG Python 节点脚本
 */

package service

import (
	"box-manage-service/config"
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

const pythonScriptSystemPrompt = `你是 DAG 工作流中的 Python 脚本生成助手。
硬性规则：
1. 输出必须是可直接交给 box-app 执行的顶层 Python 代码片段：禁止定义 def run(ctx)，禁止使用 ctx，禁止用函数包裹整段业务逻辑，也不要使用 return 作为节点输出。
2. box-app 会在执行前把 node_config.input_params 中每个参数的 name 注入为同名 Python 顶层变量；节点输入定义中的 key_name 也会作为可用变量名。直接使用这些变量，例如 rows、image、det_person，不要通过上下文对象读取。
3. box-app 会在执行后自动捕获 node_config.output_params 中每个参数 name 对应的同名顶层变量，并写回节点输出。请直接给这些变量赋值，例如 result = ...、rows = ...；不要 return，也不必手写 __OUTPUT__: 序列化标记。
4. node_inputs、node_outputs 和 node_config 是生成时的契约，不是运行时对象。节点配置中的 input_params/output_params 优先决定脚本变量名；node_outputs 的 key_name 可作为没有显式 name 时的变量名。静态配置值只能在确有需要时作为常量使用，不能假设存在 ctx.config。
5. 在代码开头显式写出所有实际使用的 import，例如使用 JSON 时必须有 import json，使用时间处理时必须有 import datetime 或 import time。优先使用 Python 标准库；禁止文件读写、网络、子进程、eval、exec、compile 和动态导入。
6. 只输出完整 Python 源码，不要 markdown 代码围栏，不要解释文字。
请先理解当前节点输入/输出参数和节点配置，再生成最小、可读、可直接执行的实现。`

var (
	// pythonScriptContextPattern 匹配「把 ctx 当作运行时上下文对象使用」的写法。
	//
	// box-app 执行 Python 节点的方式是**把脚本写成临时文件后用 python3 直接跑**
	// （node_executor.cpp:2449 execvp(python_bin, tmp_path)），
	// 变量是逐行注入成扁平名字（detections = ...），**从来没有 ctx 这个对象**。
	// 所以 ctx.get(...) / ctx['x'] 之类必然报 NameError。
	//
	// 注意不能简单匹配 `\bctx\b`：那样连 data = {'ctx': 1} 这种普通字典键、
	// 甚至注释里出现 ctx 都会被误判。这里只拦真正「当成对象用」的形态。
	pythonScriptContextPattern = regexp.MustCompile(`\bctx\s*\.|\bctx\s*\[`)
	// pythonScriptReturnPattern 匹配**顶层** return。
	// 顶层 return 在模块级别是 SyntaxError，必须拦；但函数内部的 return 是完全合法的
	// （脚本里定义 helper 函数再调用是常见写法），不能被误伤——
	// 因此按行缩进判断：缩进为 0 的 return 才是顶层 return。
	pythonScriptReturnPattern = regexp.MustCompile(`(?m)^return\b`)
)

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

// hasInlineNodeContract 判断请求体是否自带了可用的节点契约。
// 只要求有输入或输出端口即可：没有任何端口的脚本节点本身也无法生成有意义的内容。
func hasInlineNodeContract(req *PythonScriptGenerateRequest) bool {
	if req == nil {
		return false
	}
	return len(req.NodeInputs) > 0 || len(req.NodeOutputs) > 0 ||
		len(asMapSlice(req.Inputs)) > 0 || len(asMapSlice(req.Outputs)) > 0
}

type pythonScriptNodeContextResolver interface {
	Resolve(ctx context.Context, workflowID uint, nodeID string) (*PythonScriptNodeContext, error)
}

type pythonScriptGeneratorService struct {
	llm      *llmClient
	resolver pythonScriptNodeContextResolver
}

// NewPythonScriptGeneratorService 创建 Python 脚本生成服务。
func NewPythonScriptGeneratorService(cfg config.LLMConfig) PythonScriptGeneratorService {
	return &pythonScriptGeneratorService{
		llm: newLLMClient(cfg),
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
	if err := s.llm.configError(); err != nil {
		return nil, err
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
			// 节点还没入库是**最常见**的情况：用户在画布上新拖了一个 Python 节点，
			// 尚未保存就想先让 AI 把脚本写出来。此时库里当然查不到它。
			// 只要前端把节点的端口上下文一并传上来，就按请求体里的契约生成，
			// 而不是直接报 "node not found" 让用户先去保存一遍。
			if !hasInlineNodeContract(req) {
				return nil, err
			}
			log.Printf("[PythonScript] 节点 %s 未入库（workflow=%d），改用请求体中的节点契约生成脚本",
				req.NodeID, req.WorkflowID)
		} else {
			// 节点已入库时仍以后端查询结果为准，避免前端篡改节点契约。
			req.NodeInputs = nodeContext.Inputs
			req.NodeOutputs = nodeContext.Outputs
			req.NodeConfig = nodeContext.Config
		}
	}

	userPrompt, err := buildPythonScriptUserPrompt(req)
	if err != nil {
		return nil, err
	}
	if s.llm.maxRequestBytes() > 0 && int64(len(userPrompt)) > s.llm.maxRequestBytes() {
		return nil, fmt.Errorf("generation context exceeds %d bytes", s.llm.maxRequestBytes())
	}

	messages := []chatMessage{
		{Role: "system", Content: pythonScriptSystemPrompt},
		{Role: "user", Content: userPrompt},
	}
	raw, err := s.llm.complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	code := extractPythonCode(raw)
	if validationErr := validatePythonScript(code, s.llm.cfg.MaxCodeBytes); validationErr != nil {
		// DeepSeek may occasionally fall back to the historical run(ctx) format.
		// Give it two explicit repair attempts with the box-app execution contract.
		lastErr := validationErr
		for attempt := 0; attempt < 2; attempt++ {
			repairMessages := append(append([]chatMessage{}, messages...),
				chatMessage{Role: "assistant", Content: code},
				chatMessage{Role: "user", Content: pythonScriptRepairPrompt(lastErr)},
			)
			raw, repairErr := s.llm.complete(ctx, repairMessages)
			if repairErr != nil {
				return nil, fmt.Errorf("generated script validation failed: %v; repair failed: %w", lastErr, repairErr)
			}
			code = extractPythonCode(raw)
			if lastErr = validatePythonScript(code, s.llm.cfg.MaxCodeBytes); lastErr == nil {
				break
			}
		}
		if lastErr != nil {
			return nil, fmt.Errorf("generated script validation failed after repair: %w", lastErr)
		}
	}

	return &PythonScriptDraft{
		Code:       code,
		Warnings:   []string{"脚本仅完成生成和入口校验，未在服务端执行；应用前请在沙箱中试跑。"},
		UsedInputs: upstreamIDs(req.Upstream),
		Model:      s.llm.model(),
	}, nil
}

func pythonScriptRepairPrompt(validationErr error) string {
	return fmt.Sprintf(`上一次返回的 Python 代码未通过校验：%s。
请完整重写代码并只返回源码，严格遵守以下 box-app 运行契约：
- 只能返回顶层 Python 代码片段，不能出现任何 def run、def main、ctx、return 或函数包裹。
- box-app 已将 input_params 中的 name 注入为同名变量，直接使用这些变量。
- 直接给 output_params 中的 name 对应变量赋值；box-app 会自动捕获输出，不要打印 __OUTPUT__。
- 所有依赖必须在代码开头显式 import，例如 import json。
示例：
import json
rows = [row for row in source_rows if row.get("status") == status]
`, validationErr)
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
	// Older workflows may not have variable_definitions rows for a node. In
	// that case retain the node-level input/output port definitions so the
	// model still knows the names that box-app injects and captures.
	if len(result.Inputs) == 0 {
		result.Inputs = pythonScriptNodeParametersFromPorts(node.Inputs, "input")
	}
	if len(result.Outputs) == 0 {
		result.Outputs = pythonScriptNodeParametersFromPorts(node.Outputs, "output")
	}
	return result, nil
}

func pythonScriptNodeParametersFromPorts(ports models.JSONArr, direction string) []PythonScriptNodeParameter {
	parameters := make([]PythonScriptNodeParameter, 0, len(ports))
	for _, raw := range ports {
		payload, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		var parameter PythonScriptNodeParameter
		if err := json.Unmarshal(payload, &parameter); err != nil {
			continue
		}
		if strings.TrimSpace(parameter.KeyName) == "" && strings.TrimSpace(parameter.Name) != "" {
			parameter.KeyName = parameter.Name
		}
		if strings.TrimSpace(parameter.Direction) == "" {
			parameter.Direction = direction
		}
		if strings.TrimSpace(parameter.KeyName) == "" {
			continue
		}
		parameters = append(parameters, parameter)
	}
	return parameters
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
	if strings.Contains(code, "def run(") || pythonScriptContextPattern.MatchString(code) || pythonScriptReturnPattern.MatchString(code) {
		return errors.New("generated code must be a top-level snippet without def run(ctx), ctx, or top-level return")
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
