/*
 * @module repository/workflow_test_case_repository
 * @description 编排测试用例数据访问层
 */

package repository

import (
	"box-manage-service/models"
	"context"

	"gorm.io/gorm"
)

// WorkflowTestCaseRepository 测试用例 Repository 接口
type WorkflowTestCaseRepository interface {
	BaseRepository[models.WorkflowTestCase]

	// FindByWorkflowKeyName 按编排标识查询用例（跨版本复用）
	FindByWorkflowKeyName(ctx context.Context, keyName string) ([]*models.WorkflowTestCase, error)
	// FindByWorkflowKeyNames 批量按编排标识查询，用于列表页批量展示
	FindByWorkflowKeyNames(ctx context.Context, keyNames []string) ([]*models.WorkflowTestCase, error)
}

type workflowTestCaseRepository struct {
	BaseRepository[models.WorkflowTestCase]
	db *gorm.DB
}

// NewWorkflowTestCaseRepository 创建测试用例 Repository
func NewWorkflowTestCaseRepository(db *gorm.DB) WorkflowTestCaseRepository {
	return &workflowTestCaseRepository{
		BaseRepository: newBaseRepository[models.WorkflowTestCase](db),
		db:             db,
	}
}

// FindByWorkflowKeyName 按编排标识查询用例
func (r *workflowTestCaseRepository) FindByWorkflowKeyName(ctx context.Context, keyName string) ([]*models.WorkflowTestCase, error) {
	var cases []*models.WorkflowTestCase
	err := r.db.WithContext(ctx).
		Where("workflow_key_name = ?", keyName).
		Order("id ASC").
		Find(&cases).Error
	return cases, err
}

// FindByWorkflowKeyNames 批量按编排标识查询
func (r *workflowTestCaseRepository) FindByWorkflowKeyNames(ctx context.Context, keyNames []string) ([]*models.WorkflowTestCase, error) {
	if len(keyNames) == 0 {
		return []*models.WorkflowTestCase{}, nil
	}
	var cases []*models.WorkflowTestCase
	err := r.db.WithContext(ctx).
		Where("workflow_key_name IN ?", keyNames).
		Order("workflow_key_name ASC, id ASC").
		Find(&cases).Error
	return cases, err
}
