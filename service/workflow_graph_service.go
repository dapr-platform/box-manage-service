/*
 * @module service/workflow_graph_service
 * @description 编排图的补全（normalize）与校验（validate）服务。
 *              供手工导入、AI 生成结果导入共用：导入走 normalize（补全 + 校验），
 *              校验按钮走 validate，保存走 ValidateStructure。
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// WorkflowGraphService 编排图补全与校验服务接口。
type WorkflowGraphService interface {
	// Normalize 把完整或精简的 LogicFlow JSON 补全为可导入的画布 JSON，并返回校验报告。
	Normalize(ctx context.Context, raw json.RawMessage) (*WorkflowNormalizeResult, error)
	// Validate 校验完整或精简的 LogicFlow JSON。
	Validate(ctx context.Context, raw json.RawMessage) (*ValidationReport, error)
}

// WorkflowNormalizeResult 补全结果。
type WorkflowNormalizeResult struct {
	Graph  *graphDocument    `json:"graph"`
	Report *ValidationReport `json:"report"`
}

type workflowGraphService struct {
	nodeTemplateRepo repository.NodeTemplateRepository
	variableDefRepo  repository.VariableDefinitionRepository
}

// NewWorkflowGraphService 创建编排图补全/校验服务。
func NewWorkflowGraphService(db *gorm.DB) WorkflowGraphService {
	service := &workflowGraphService{}
	if db != nil {
		service.nodeTemplateRepo = repository.NewNodeTemplateRepository(db)
		service.variableDefRepo = repository.NewVariableDefinitionRepository(db)
	}
	return service
}

// prepare 加载启用节点模板（含变量定义）并构建校验器。
func (s *workflowGraphService) prepare(ctx context.Context) (*WorkflowValidator, map[string]*models.NodeTemplate, error) {
	if s.nodeTemplateRepo == nil {
		return nil, nil, errors.New("workflow graph service is not configured with a database")
	}
	templates, err := loadEnabledTemplatesWithVariables(ctx, s.nodeTemplateRepo, s.variableDefRepo)
	if err != nil {
		return nil, nil, err
	}
	return NewWorkflowValidator(templates), buildTemplateIndex(templates), nil
}

// Normalize 补全 + 校验。
func (s *workflowGraphService) Normalize(ctx context.Context, raw json.RawMessage) (*WorkflowNormalizeResult, error) {
	validator, templateIndex, err := s.prepare(ctx)
	if err != nil {
		return nil, err
	}
	document, err := parseGraph(raw)
	if err != nil {
		return nil, err
	}
	simple := simplifyGraph(document)
	completed, err := completeGraph(simple, completeOptions{Templates: templateIndex})
	if err != nil {
		return nil, err
	}
	return &WorkflowNormalizeResult{Graph: completed, Report: validator.Validate(completed, simple)}, nil
}

// Validate 只校验，不补全。
func (s *workflowGraphService) Validate(ctx context.Context, raw json.RawMessage) (*ValidationReport, error) {
	validator, _, err := s.prepare(ctx)
	if err != nil {
		return nil, err
	}
	document, err := parseGraph(raw)
	if err != nil {
		return nil, err
	}
	return validator.Validate(document, simplifyGraph(document)), nil
}

// loadEnabledTemplatesWithVariables 查询启用中的节点模板并填充变量定义。
// repository 层的 FindEnabled 不会加载 Variables，而提示词的「基础节点说明」
// 与校验器的固定参数检查都依赖它，因此统一在这里补齐。
func loadEnabledTemplatesWithVariables(
	ctx context.Context,
	repo repository.NodeTemplateRepository,
	variableRepo repository.VariableDefinitionRepository,
) ([]*models.NodeTemplate, error) {
	if repo == nil {
		return nil, errors.New("node template repository is not configured")
	}
	templates, err := repo.FindEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询节点模板失败: %w", err)
	}
	if variableRepo == nil {
		return templates, nil
	}
	for _, template := range templates {
		if template == nil {
			continue
		}
		variables, err := variableRepo.FindByNodeTemplateID(ctx, template.ID)
		if err != nil {
			return nil, fmt.Errorf("查询模板 %d 的变量定义失败: %w", template.ID, err)
		}
		template.Variables = make([]models.VariableDefinition, len(variables))
		for i, variable := range variables {
			template.Variables[i] = *variable
		}
	}
	return templates, nil
}

// buildTemplateIndex 以 type_key 为键建立模板索引。
func buildTemplateIndex(templates []*models.NodeTemplate) map[string]*models.NodeTemplate {
	index := make(map[string]*models.NodeTemplate, len(templates))
	for _, template := range templates {
		if template != nil {
			index[template.TypeKey] = template
		}
	}
	return index
}
