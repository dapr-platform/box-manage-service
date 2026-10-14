/*
 * @module service/workflow_validation_hook
 * @description 把统一校验器接入工作流的保存与发布流程。
 *              - 保存草稿（Create/Update）：只记录日志，不阻断。
 *                草稿允许是半成品（例如刚放下开始/结束节点还没连线），硬性阻断会让用户无法保存。
 *              - 发布（Publish）：**强门禁**，统一校验器报错即拒绝发布（见 blockPublishOnValidationError）。
 *              - 显式校验接口（POST /api/v1/workflows/validate）与导入（normalize）：
 *                使用同一份校验器并返回完整报告，作为用户修正依据。
 *              原有的定义表校验（必须有开始/结束节点、模板存在、连线端点存在）保持不变。
 */

package service

import (
	"box-manage-service/models"
	"context"
	"encoding/json"
	"log"
	"strings"
)

// blockPublishOnValidationError 是否把统一校验器作为发布的强门禁。
// 开启后，校验不通过的编排一律不允许发布；草稿保存不受影响。
const blockPublishOnValidationError = true

// logWorkflowGraphIssues 用统一校验器检查工作流视图 JSON 并记录结果。
// 视图数据为空或无法解析时直接返回，交由原有的定义表校验兜底。
func (s *workflowService) logWorkflowGraphIssues(ctx context.Context, workflow *models.Workflow, action string) {
	report := s.checkWorkflowGraph(ctx, workflow, false)
	if report == nil || (len(report.Errors) == 0 && len(report.Warnings) == 0) {
		return
	}
	log.Printf("[WorkflowValidate][%s] workflow=%s 发现 %d 个错误、%d 个警告：%s",
		action, workflow.KeyName, len(report.Errors), len(report.Warnings), formatValidationIssues(report))
}

// WorkflowValidationError 表示统一校验器判定编排不合法。
// 控制器据此返回 400（客户端可修正），而不是 500。
type WorkflowValidationError struct {
	Issues []ValidationIssue
}

// Error 实现 error；消息为各条错误的拼接，便于直接展示给用户。
func (e *WorkflowValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "编排校验未通过"
	}
	messages := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		messages = append(messages, issue.Message)
	}
	return strings.Join(messages, "；")
}

// Report 便于调用方取出完整报告（含 warnings 与定位信息）。
func (e *WorkflowValidationError) Report() *ValidationReport {
	if e == nil {
		return &ValidationReport{Errors: []ValidationIssue{}, Warnings: []ValidationIssue{}}
	}
	return &ValidationReport{Valid: false, Errors: e.Issues, Warnings: []ValidationIssue{}}
}

// ensurePublishable 发布前的统一校验；作为强门禁时校验不通过即阻断发布。
func (s *workflowService) ensurePublishable(ctx context.Context, workflow *models.Workflow) error {
	s.logWorkflowGraphIssues(ctx, workflow, "publish")
	if !blockPublishOnValidationError {
		return nil
	}
	// 发布要求全图连通：所有节点都能从开始节点到达，并且都能到达结束节点。
	report := s.checkWorkflowGraph(ctx, workflow, true)
	if report == nil || report.Valid {
		return nil
	}
	return &WorkflowValidationError{Issues: report.Errors}
}

// checkWorkflowGraph 执行统一校验；strict 为 true 时要求全图连通。
// 返回 nil 表示视图数据不可用（回退到定义表校验）。
func (s *workflowService) checkWorkflowGraph(ctx context.Context, workflow *models.Workflow, strict bool) *ValidationReport {
	if workflow == nil {
		return nil
	}
	view := strings.TrimSpace(workflow.StructureJSONView)
	if view == "" {
		return nil
	}
	// 必须带上模板变量，否则固定参数检查会失去依据。
	templates, err := loadEnabledTemplatesWithVariables(ctx, s.nodeTemplateRepo, s.variableDefRepo)
	if err != nil {
		log.Printf("[WorkflowValidate] 查询节点模板失败: %v", err)
		return nil
	}
	document, err := parseGraph(json.RawMessage(view))
	if err != nil {
		// 视图可能是旧的或手写的结构，交给定义表校验判断。
		return nil
	}
	validator := NewWorkflowValidator(templates)
	if strict {
		return validator.Validate(document, simplifyGraph(document))
	}
	return validator.ValidateDraft(document, simplifyGraph(document))
}

// formatValidationIssues 把校验结果拼成一行便于返回与记录。
func formatValidationIssues(report *ValidationReport) string {
	if report == nil {
		return "编排校验未通过"
	}
	messages := make([]string, 0, len(report.Errors)+len(report.Warnings))
	for _, issue := range report.Errors {
		messages = append(messages, issue.Message)
	}
	if len(messages) == 0 {
		for _, issue := range report.Warnings {
			messages = append(messages, issue.Message)
		}
	}
	if len(messages) == 0 {
		return "编排校验未通过"
	}
	return strings.Join(messages, "；")
}
