/*
 * @module service/workflow_test_case_ai
 * @description AI 生成测试用例（只返回草稿，不保存）。
 *
 * **关键原则：用例从需求出发，而不是从编排出发。**
 * 如果让模型看着脚本写预期值，测试只会「证明代码等于代码」。
 * 因此提示词只给节点与输出的**名称**用于拼断言路径，绝不给脚本内容；
 * 预期值必须由业务需求推导。
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

// workflowTestCaseSystemPrompt 用例生成的角色与硬性要求。
const workflowTestCaseSystemPrompt = `你是 AI 盒子业务编排的测试用例设计助手。
你要根据**业务需求**设计一组逐帧测试用例，用来验证一张编排是否真正满足需求。
只输出 JSON，不要输出解释文字，不要输出 markdown 代码围栏。

硬性规则：
1. 用例必须从业务需求推导预期结果，不要假设编排实现一定正确。你只被告知节点名称和输出名称（用于拼断言路径），看不到脚本内容，这是刻意的：请按「需求要求的结果应该是什么」来写期望值。
2. 每帧的 inputs 是开始节点的输入变量，通常是 detections（检测结果数组，元素含 class_id、confidence）和 image_base64。
3. 断言用 path 指向 <节点id>.<输出key>，或只用 node + executed 判断某节点本帧是否执行（用于验证条件分支）。
4. 必须覆盖这些阶段（按需求取舍，但至少包含正常触发与恢复）：
   - 正常触发前的过渡帧
   - 恰好达到阈值的那一帧（边界）
   - 阈值前一帧（差一点不触发）
   - 中途漏检后恢复
   - 触发后的恢复/解除
   - 重复触发（例如目标数量增加）
5. 需要跨帧累计的场景，用 repeat 表示「同样的输入重复 N 帧」，此时断言默认只在最后一帧判断；需要每帧都判断时设置 expect_each 为 true。
6. label 写清楚该帧在验证什么，失败信息会直接展示给用户。
7. 只输出一个 JSON 对象，字段为 name、description、initial_context、frames。
8. 不要输出注释、不要输出多余字段。`

// workflowTestCaseUserPromptTemplate 用户提示词模板。
const workflowTestCaseUserPromptTemplate = `【业务需求】
%s

%s【编排中的节点与输出（仅用于拼断言路径，不要据此反推预期值）】
%s

【用例 JSON 格式说明】
{
  "name": "用例名称",
  "description": "覆盖了哪些阶段",
  "initial_context": {},
  "frames": [
    {
      "label": "有人 1~3 帧，未进入入侵状态",
      "repeat": 3,
      "inputs": { "detections": [{ "class_id": 0, "confidence": 0.92 }], "image_base64": "" },
      "expect": [
        { "path": "python_script-005.is_intrusion_detected", "op": "eq", "value": false },
        { "node": "wechat_work-007", "executed": false }
      ]
    },
    {
      "label": "第 4 帧进入入侵状态",
      "inputs": { "detections": [{ "class_id": 0, "confidence": 0.92 }] },
      "expect": [{ "path": "python_script-005.is_intrusion_detected", "op": "eq", "value": true }]
    }
  ]
}

【可用的 op】
eq、ne、gt、gte、lt、lte、contains、exists、not_exists

请输出一份覆盖充分、label 清晰、预期值由需求推导的用例 JSON。`

// WorkflowTestCaseAIService AI 生成测试用例服务接口
type WorkflowTestCaseAIService interface {
	Generate(ctx context.Context, req *models.WorkflowTestCaseAIRequest) (*models.WorkflowTestCaseAIDraft, error)
}

type workflowTestCaseAIService struct {
	llm          *llmClient
	workflowRepo repository.WorkflowRepository
	nodeDefRepo  repository.NodeDefinitionRepository
	nodeTplRepo  repository.NodeTemplateRepository
	varDefRepo   repository.VariableDefinitionRepository
}

// NewWorkflowTestCaseAIService 创建 AI 用例生成服务
func NewWorkflowTestCaseAIService(cfg config.LLMConfig, db *gorm.DB) WorkflowTestCaseAIService {
	service := &workflowTestCaseAIService{llm: newLLMClient(cfg)}
	if db == nil {
		return service
	}
	service.workflowRepo = repository.NewWorkflowRepository(db)
	service.nodeDefRepo = repository.NewNodeDefinitionRepository(db)
	service.nodeTplRepo = repository.NewNodeTemplateRepository(db)
	service.varDefRepo = repository.NewVariableDefinitionRepository(db)
	return service
}

// Generate 生成用例草稿（不保存）。
func (s *workflowTestCaseAIService) Generate(ctx context.Context, req *models.WorkflowTestCaseAIRequest) (*models.WorkflowTestCaseAIDraft, error) {
	if req == nil || strings.TrimSpace(req.Description) == "" {
		return nil, errors.New("description is required")
	}
	if err := s.llm.configError(); err != nil {
		return nil, err
	}
	if s.workflowRepo == nil {
		return nil, errors.New("workflow test case AI service is not configured with a database")
	}

	nodeOutline, err := s.buildNodeOutline(ctx, req.WorkflowID)
	if err != nil {
		return nil, err
	}
	assumptionText := ""
	if len(req.Assumptions) > 0 {
		assumptionText = "【生成编排时采用的假设（可作为阈值参考）】\n- " + strings.Join(req.Assumptions, "\n- ") + "\n\n"
	}
	userPrompt := fmt.Sprintf(workflowTestCaseUserPromptTemplate, strings.TrimSpace(req.Description), assumptionText, nodeOutline)
	if limit := s.llm.maxRequestBytes(); limit > 0 && int64(len(userPrompt)) > limit {
		return nil, fmt.Errorf("提示词长度 %d 字节，超过上限 %d 字节", len(userPrompt), limit)
	}

	raw, err := s.llm.complete(ctx, []chatMessage{
		{Role: "system", Content: workflowTestCaseSystemPrompt},
		{Role: "user", Content: userPrompt},
	})
	if err != nil {
		return nil, err
	}

	payload := extractJSONObject(raw)
	if payload == "" {
		return nil, errors.New("模型返回的内容不是合法 JSON")
	}
	var content models.WorkflowTestCaseContent
	if err := json.Unmarshal([]byte(payload), &content); err != nil {
		return nil, fmt.Errorf("模型返回的用例不是合法 JSON: %w", err)
	}
	if len(content.Frames) == 0 {
		return nil, errors.New("模型返回的用例没有任何帧")
	}
	return &models.WorkflowTestCaseAIDraft{
		Name:        firstNonEmptyString(content.Name, "AI 生成用例"),
		Description: content.Description,
		Content:     content,
		Warnings: []string{
			"用例由大模型根据需求生成，预期值可能与实际业务约定不一致，请先人工核对再保存。",
		},
		Model: s.llm.model(),
	}, nil
}

// buildNodeOutline 生成「节点与输出名称」清单。
// 刻意只给名称（用于拼断言路径），不给脚本内容，避免模型照着实现写期望值。
func (s *workflowTestCaseAIService) buildNodeOutline(ctx context.Context, workflowID uint) (string, error) {
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return "", fmt.Errorf("查询编排失败: %w", err)
	}
	if workflow == nil {
		return "", fmt.Errorf("编排 %d 不存在", workflowID)
	}
	nodes, err := s.nodeDefRepo.FindByWorkflowID(ctx, workflowID)
	if err != nil {
		return "", fmt.Errorf("查询编排节点失败: %w", err)
	}
	variables, err := s.varDefRepo.FindByWorkflowID(ctx, workflowID)
	if err != nil {
		return "", fmt.Errorf("查询编排变量失败: %w", err)
	}

	// 按节点归集输出 key（开始节点用 inputs，它代表用例的输入契约）
	outputsByNode := map[string][]string{}
	inputsByNode := map[string][]string{}
	for _, variable := range variables {
		entry := variable.KeyName
		if variable.Name != "" {
			entry = fmt.Sprintf("%s（%s）", variable.KeyName, variable.Name)
		}
		if variable.Direction == models.VariableDirectionOutput {
			outputsByNode[variable.NodeID] = append(outputsByNode[variable.NodeID], entry)
		} else {
			inputsByNode[variable.NodeID] = append(inputsByNode[variable.NodeID], entry)
		}
	}

	var builder strings.Builder
	builder.WriteString("（type 为 start 的节点即用例 inputs 要填写的变量来源）\n")
	// 开始节点优先展示，便于模型知道可用的输入
	ordered := make([]*models.NodeDefinition, 0, len(nodes))
	for _, node := range nodes {
		if node.TypeKey == "start" {
			ordered = append(ordered, node)
		}
	}
	for _, node := range nodes {
		if node.TypeKey != "start" {
			ordered = append(ordered, node)
		}
	}
	for _, node := range ordered {
		name := firstNonEmptyString(node.NodeName, node.TypeName, node.TypeKey)
		builder.WriteString(fmt.Sprintf("- %s | type=%s | id=%s\n", name, node.TypeKey, node.NodeID))
		if list := inputsByNode[node.NodeID]; len(list) > 0 {
			builder.WriteString("    输入: " + strings.Join(list, "、") + "\n")
		}
		if list := outputsByNode[node.NodeID]; len(list) > 0 {
			builder.WriteString("    输出: " + strings.Join(list, "、") + "\n")
		}
	}
	return builder.String(), nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
