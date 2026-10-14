/*
 * @module service/workflow_test_case_service
 * @description 编排测试用例的增删改查。
 *              用例按 workflow_key_name 关联，编排升级版本后仍可复用，作为回归测试。
 *              测试结果不入库（YAGNI，需要审计时再加）。
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// WorkflowTestCaseService 测试用例服务接口
type WorkflowTestCaseService interface {
	// ListByWorkflowID 查询某编排（按 key_name）的全部用例
	ListByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowTestCase, error)
	// Create 新建用例
	Create(ctx context.Context, workflowID uint, testCase *models.WorkflowTestCase) (*models.WorkflowTestCase, error)
	// Update 修改用例
	Update(ctx context.Context, caseID uint, testCase *models.WorkflowTestCase) (*models.WorkflowTestCase, error)
	// Delete 删除用例
	Delete(ctx context.Context, caseID uint) error
	// GetByID 查询单个用例
	GetByID(ctx context.Context, caseID uint) (*models.WorkflowTestCase, error)
}

type workflowTestCaseService struct {
	testCaseRepo repository.WorkflowTestCaseRepository
	workflowRepo repository.WorkflowRepository
}

// NewWorkflowTestCaseService 创建测试用例服务
func NewWorkflowTestCaseService(repoManager repository.RepositoryManager) WorkflowTestCaseService {
	if repoManager == nil {
		return &workflowTestCaseService{}
	}
	return &workflowTestCaseService{
		testCaseRepo: repoManager.WorkflowTestCase(),
		workflowRepo: repoManager.Workflow(),
	}
}

// resolveKeyName 由编排 ID 查出 key_name；用例按 key_name 关联。
func (s *workflowTestCaseService) resolveKeyName(ctx context.Context, workflowID uint) (string, error) {
	if s.workflowRepo == nil {
		return "", errors.New("workflow repository is not configured")
	}
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return "", fmt.Errorf("查询编排失败: %w", err)
	}
	if workflow == nil {
		return "", fmt.Errorf("编排 %d 不存在", workflowID)
	}
	return workflow.KeyName, nil
}

// ListByWorkflowID 查询某编排的全部用例
func (s *workflowTestCaseService) ListByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowTestCase, error) {
	if s.testCaseRepo == nil {
		return nil, errors.New("workflow test case repository is not configured")
	}
	keyName, err := s.resolveKeyName(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	return s.testCaseRepo.FindByWorkflowKeyName(ctx, keyName)
}

// GetByID 查询单个用例
func (s *workflowTestCaseService) GetByID(ctx context.Context, caseID uint) (*models.WorkflowTestCase, error) {
	if s.testCaseRepo == nil {
		return nil, errors.New("workflow test case repository is not configured")
	}
	return s.testCaseRepo.GetByID(ctx, caseID)
}

// Create 新建用例
func (s *workflowTestCaseService) Create(ctx context.Context, workflowID uint, testCase *models.WorkflowTestCase) (*models.WorkflowTestCase, error) {
	if s.testCaseRepo == nil {
		return nil, errors.New("workflow test case repository is not configured")
	}
	if testCase == nil {
		return nil, errors.New("用例内容不能为空")
	}
	keyName, err := s.resolveKeyName(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	testCase.ID = 0
	testCase.WorkflowKeyName = keyName
	if err := normalizeTestCase(testCase); err != nil {
		return nil, err
	}
	if err := s.testCaseRepo.Create(ctx, testCase); err != nil {
		return nil, fmt.Errorf("保存用例失败: %w", err)
	}
	return testCase, nil
}

// Update 修改用例
func (s *workflowTestCaseService) Update(ctx context.Context, caseID uint, testCase *models.WorkflowTestCase) (*models.WorkflowTestCase, error) {
	if s.testCaseRepo == nil {
		return nil, errors.New("workflow test case repository is not configured")
	}
	if testCase == nil {
		return nil, errors.New("用例内容不能为空")
	}
	existing, err := s.testCaseRepo.GetByID(ctx, caseID)
	if err != nil {
		return nil, fmt.Errorf("查询用例失败: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("用例 %d 不存在", caseID)
	}
	if err := normalizeTestCase(testCase); err != nil {
		return nil, err
	}
	// 只更新内容字段，key_name 与创建者保持不变
	existing.Name = testCase.Name
	existing.Description = testCase.Description
	existing.Content = testCase.Content
	if testCase.UpdatedBy > 0 {
		existing.UpdatedBy = testCase.UpdatedBy
	}
	if err := s.testCaseRepo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("更新用例失败: %w", err)
	}
	return existing, nil
}

// Delete 删除用例
func (s *workflowTestCaseService) Delete(ctx context.Context, caseID uint) error {
	if s.testCaseRepo == nil {
		return errors.New("workflow test case repository is not configured")
	}
	existing, err := s.testCaseRepo.GetByID(ctx, caseID)
	if err != nil {
		return fmt.Errorf("查询用例失败: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("用例 %d 不存在", caseID)
	}
	return s.testCaseRepo.Delete(ctx, caseID)
}

// normalizeTestCase 校验并归一化用例内容：必须能解析、必须至少有一帧，
// 并把 content 中的 name/description 与顶层字段对齐。
func normalizeTestCase(testCase *models.WorkflowTestCase) error {
	if strings.TrimSpace(testCase.Name) == "" {
		return errors.New("用例名称不能为空")
	}
	content := strings.TrimSpace(testCase.Content)
	if content == "" {
		return errors.New("用例内容不能为空")
	}
	var parsed models.WorkflowTestCaseContent
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return fmt.Errorf("用例内容不是合法 JSON: %w", err)
	}
	if len(parsed.Frames) == 0 {
		return errors.New("用例至少需要一帧")
	}
	if strings.TrimSpace(parsed.Name) == "" {
		parsed.Name = testCase.Name
	}
	if strings.TrimSpace(parsed.Description) == "" {
		parsed.Description = testCase.Description
	}
	normalized, err := json.Marshal(parsed)
	if err != nil {
		return fmt.Errorf("序列化用例失败: %w", err)
	}
	testCase.Content = string(normalized)
	return nil
}
