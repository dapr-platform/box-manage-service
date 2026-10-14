/*
 * @module service/workflow_ai_service
 * @description AI 快速编排：单轮生成。
 *              组装提示词 → 调用大模型 → 解析（问题列表 或 精简编排 JSON）
 *              → 补全为完整 LogicFlow JSON → 统一校验 → 返回结果与校验报告。
 *              本服务无状态，多轮修复循环由前端驱动，每次请求只调用一次大模型。
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
	"strings"

	"gorm.io/gorm"
)

// WorkflowAIGenerateRequest 单轮生成请求。
type WorkflowAIGenerateRequest struct {
	// TemplateWorkflowID 参考模板编排 ID，可选。
	TemplateWorkflowID uint `json:"template_workflow_id,omitempty"`
	// Description 业务需求，必填。
	Description string `json:"description"`
	// Clarifications 用户对上一轮反问的回答。
	Clarifications []workflowAIClarification `json:"clarifications,omitempty"`
	// Previous 上一轮结果与错误，仅修复轮次携带。
	Previous *workflowAIPrevious `json:"previous,omitempty"`
}

// WorkflowAIGenerateResult 单轮生成结果。
type WorkflowAIGenerateResult struct {
	// Status generated / need_clarification
	Status string `json:"status"`
	// Name 模型建议的编排名称。
	Name string `json:"name,omitempty"`
	// Graph 补全后的完整 LogicFlow JSON，可直接导入画布。
	Graph *graphDocument `json:"graph,omitempty"`
	// Raw 模型返回的精简 JSON 文本，修复轮次需要回传。
	Raw string `json:"raw,omitempty"`
	// Report 统一校验器的结果。
	Report *ValidationReport `json:"report,omitempty"`
	// Summary 模型给出的业务逻辑说明。
	Summary string `json:"summary,omitempty"`
	// Assumptions 模型自行采用的假设。
	Assumptions []string `json:"assumptions,omitempty"`
	// Questions 反问列表（status = need_clarification 时）。
	Questions []workflowAIQuestion `json:"questions,omitempty"`
	// Model 实际使用的模型名。
	Model string `json:"model,omitempty"`
	// TemplateName 参考模板名称，便于前端展示。
	TemplateName string `json:"template_name,omitempty"`
}

// WorkflowAIService AI 快速编排服务接口。
type WorkflowAIService interface {
	Generate(ctx context.Context, req *WorkflowAIGenerateRequest) (*WorkflowAIGenerateResult, error)
}

type workflowAIService struct {
	llm              *llmClient
	workflowRepo     repository.WorkflowRepository
	nodeDefRepo      repository.NodeDefinitionRepository
	lineDefRepo      repository.LineDefinitionRepository
	variableDefRepo  repository.VariableDefinitionRepository
	nodeTemplateRepo repository.NodeTemplateRepository
	validator        *WorkflowValidator
}

// NewWorkflowAIService 创建 AI 快速编排服务。
func NewWorkflowAIService(cfg config.LLMConfig, db *gorm.DB) WorkflowAIService {
	service := &workflowAIService{llm: newLLMClient(cfg)}
	if db == nil {
		return service
	}
	service.workflowRepo = repository.NewWorkflowRepository(db)
	service.nodeDefRepo = repository.NewNodeDefinitionRepository(db)
	service.lineDefRepo = repository.NewLineDefinitionRepository(db)
	service.variableDefRepo = repository.NewVariableDefinitionRepository(db)
	service.nodeTemplateRepo = repository.NewNodeTemplateRepository(db)
	return service
}

// Generate 执行一轮生成或修复。
func (s *workflowAIService) Generate(ctx context.Context, req *WorkflowAIGenerateRequest) (*WorkflowAIGenerateResult, error) {
	if req == nil || strings.TrimSpace(req.Description) == "" {
		return nil, errors.New("description is required")
	}
	if err := s.llm.configError(); err != nil {
		return nil, err
	}
	if s.nodeTemplateRepo == nil {
		return nil, errors.New("workflow AI service is not configured with a database")
	}

	// 必须把模板变量一并查出来：提示词的「基础节点说明」和校验器的固定参数检查都依赖它。
	templates, err := loadEnabledTemplatesWithVariables(ctx, s.nodeTemplateRepo, s.variableDefRepo)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, errors.New("没有启用中的节点模板，无法生成编排")
	}
	s.validator = NewWorkflowValidator(templates)

	templateGraph, templateName, err := s.loadTemplate(ctx, req.TemplateWorkflowID)
	if err != nil {
		return nil, err
	}

	userPrompt, err := buildWorkflowAIPrompt(promptContext{
		Templates:      templates,
		TemplateGraph:  templateGraph,
		TemplateName:   templateName,
		Description:    req.Description,
		Clarifications: req.Clarifications,
		Previous:       req.Previous,
		MaxBytes:       s.llm.maxRequestBytes(),
	})
	if err != nil {
		return nil, err
	}

	messages := []chatMessage{
		{Role: "system", Content: workflowAISystemPrompt},
		{Role: "user", Content: userPrompt},
	}
	raw, err := s.llm.complete(ctx, messages)
	if err != nil {
		return nil, err
	}
	result := &WorkflowAIGenerateResult{Model: s.llm.model(), TemplateName: templateName}
	return s.interpret(raw, result, templates)
}

// loadTemplate 读取参考模板编排并精简。
func (s *workflowAIService) loadTemplate(ctx context.Context, workflowID uint) (*SimpleGraph, string, error) {
	if workflowID == 0 {
		return nil, "", nil
	}
	if s.workflowRepo == nil {
		return nil, "", errors.New("workflow repository is not configured")
	}
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return nil, "", fmt.Errorf("查询参考模板编排失败: %w", err)
	}
	if workflow == nil {
		return nil, "", fmt.Errorf("参考模板编排 %d 不存在", workflowID)
	}
	// 视图字段缺失时用定义表补齐，保证模板可用。
	if strings.TrimSpace(workflow.StructureJSONView) == "" {
		nodes, err := s.nodeDefRepo.FindByWorkflowID(ctx, workflowID)
		if err != nil {
			return nil, "", fmt.Errorf("查询参考模板节点失败: %w", err)
		}
		lines, err := s.lineDefRepo.FindByWorkflowID(ctx, workflowID)
		if err != nil {
			return nil, "", fmt.Errorf("查询参考模板连线失败: %w", err)
		}
		variables, err := s.variableDefRepo.FindByWorkflowID(ctx, workflowID)
		if err != nil {
			return nil, "", fmt.Errorf("查询参考模板变量失败: %w", err)
		}
		workflow.Nodes = make([]models.NodeDefinition, len(nodes))
		for i, node := range nodes {
			workflow.Nodes[i] = *node
		}
		workflow.Lines = make([]models.LineDefinition, len(lines))
		for i, line := range lines {
			workflow.Lines[i] = *line
		}
		workflow.Variables = make([]models.VariableDefinition, len(variables))
		for i, variable := range variables {
			workflow.Variables[i] = *variable
		}
	}
	document, err := graphFromWorkflow(workflow)
	if err != nil {
		return nil, "", err
	}
	return simplifyGraph(document), workflow.Name, nil
}

// interpret 解析模型返回内容，产出反问或已校验的编排。
func (s *workflowAIService) interpret(raw string, result *WorkflowAIGenerateResult, templates []*models.NodeTemplate) (*WorkflowAIGenerateResult, error) {
	payload := extractJSONObject(raw)
	if payload == "" {
		result.Status = "generated"
		result.Raw = strings.TrimSpace(raw)
		result.Report = &ValidationReport{
			Valid:  false,
			Errors: []ValidationIssue{{Code: "invalid_json", Message: "模型返回的内容不是合法 JSON，请重新生成"}},
		}
		return result, nil
	}
	var envelope struct {
		Status      string               `json:"status"`
		Name        string               `json:"name"`
		Summary     string               `json:"summary"`
		Assumptions []string             `json:"assumptions"`
		Questions   []workflowAIQuestion `json:"questions"`
		Nodes       []SimpleNode         `json:"nodes"`
		Edges       []SimpleEdge         `json:"edges"`
	}
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		result.Status = "generated"
		result.Raw = payload
		result.Report = &ValidationReport{
			Valid:  false,
			Errors: []ValidationIssue{{Code: "invalid_json", Message: fmt.Sprintf("模型返回内容不是合法 JSON：%v", err)}},
		}
		return result, nil
	}

	if envelope.Status == "need_clarification" || (len(envelope.Nodes) == 0 && len(envelope.Questions) > 0) {
		questions := envelope.Questions
		if len(questions) > maxClarificationQuestions {
			questions = questions[:maxClarificationQuestions]
		}
		result.Status = "need_clarification"
		result.Questions = questions
		result.Raw = payload
		return result, nil
	}

	result.Status = "generated"
	result.Raw = payload
	result.Name = strings.TrimSpace(envelope.Name)
	result.Summary = strings.TrimSpace(envelope.Summary)
	result.Assumptions = envelope.Assumptions

	simple := &SimpleGraph{Nodes: envelope.Nodes, Edges: envelope.Edges}
	templateIndex := buildTemplateIndex(templates)
	document, err := completeGraph(simple, completeOptions{Templates: templateIndex})
	if err != nil {
		result.Report = &ValidationReport{
			Valid:  false,
			Errors: []ValidationIssue{{Code: "graph_incomplete", Message: err.Error()}},
		}
		return result, nil
	}
	result.Graph = document
	if s.validator == nil {
		s.validator = NewWorkflowValidator(templates)
	}
	result.Report = s.validator.Validate(document, simple)
	return result, nil
}

// maxClarificationQuestions 单轮反问的问题数量上限。
const maxClarificationQuestions = 5

// extractJSONObject 从模型输出中提取 JSON 对象文本，兼容 markdown 代码围栏与前后解释文字。
func extractJSONObject(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if start := strings.Index(text, "```"); start >= 0 {
		bodyStart := strings.IndexByte(text[start+3:], '\n')
		if bodyStart >= 0 {
			bodyStart += start + 3 + 1
			if end := strings.Index(text[bodyStart:], "```"); end >= 0 {
				candidate := strings.TrimSpace(text[bodyStart : bodyStart+end])
				if json.Valid([]byte(candidate)) {
					return candidate
				}
				text = candidate
			}
		}
	}
	start := strings.IndexByte(text, '{')
	if start < 0 {
		return ""
	}
	// 从第一个 { 开始按括号配对截取，避免尾部解释文字破坏 JSON。
	depth := 0
	inString := false
	escaped := false
	for index := start; index < len(text); index++ {
		character := text[index]
		if inString {
			switch {
			case escaped:
				escaped = false
			case character == '\\':
				escaped = true
			case character == '"':
				inString = false
			}
			continue
		}
		switch character {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				candidate := text[start : index+1]
				if json.Valid([]byte(candidate)) {
					return candidate
				}
				return ""
			}
		}
	}
	return ""
}
