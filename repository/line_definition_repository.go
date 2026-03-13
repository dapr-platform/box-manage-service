/*
 * @module repository/line_definition_repository
 * @description 连接线定义Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> LineDefinitionRepository -> GORM -> 数据库
 * @rules 实现连接线定义的CRUD操作、工作流关联查询等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.3节
 */

package repository

import (
	"box-manage-service/models"
	"context"

	"gorm.io/gorm"
)

// LineDefinitionRepository 连接线定义Repository接口
type LineDefinitionRepository interface {
	BaseRepository[models.LineDefinition]

	// 基础查询
	FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.LineDefinition, error)
	FindBySourceNodeDefID(ctx context.Context, sourceNodeDefID uint) ([]*models.LineDefinition, error)
	FindByTargetNodeDefID(ctx context.Context, targetNodeDefID uint) ([]*models.LineDefinition, error)

	// 批量操作
	CreateBatchForWorkflow(ctx context.Context, workflowID uint, definitions []*models.LineDefinition) error
	DeleteByWorkflowID(ctx context.Context, workflowID uint) error
}

// lineDefinitionRepository 连接线定义Repository实现
type lineDefinitionRepository struct {
	BaseRepository[models.LineDefinition]
	db *gorm.DB
}

// NewLineDefinitionRepository 创建LineDefinition Repository实例
func NewLineDefinitionRepository(db *gorm.DB) LineDefinitionRepository {
	return &lineDefinitionRepository{
		BaseRepository: newBaseRepository[models.LineDefinition](db),
		db:             db,
	}
}

// FindByWorkflowID 根据工作流ID查找连接线定义
func (r *lineDefinitionRepository) FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.LineDefinition, error) {
	var definitions []*models.LineDefinition
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("id ASC").
		Find(&definitions).Error
	return definitions, err
}

// FindBySourceNodeDefID 根据源节点定义ID查找连接线定义
func (r *lineDefinitionRepository) FindBySourceNodeDefID(ctx context.Context, sourceNodeDefID uint) ([]*models.LineDefinition, error) {
	var definitions []*models.LineDefinition
	err := r.db.WithContext(ctx).
		Where("source_node_def_id = ?", sourceNodeDefID).
		Find(&definitions).Error
	return definitions, err
}

// FindByTargetNodeDefID 根据目标节点定义ID查找连接线定义
func (r *lineDefinitionRepository) FindByTargetNodeDefID(ctx context.Context, targetNodeDefID uint) ([]*models.LineDefinition, error) {
	var definitions []*models.LineDefinition
	err := r.db.WithContext(ctx).
		Where("target_node_def_id = ?", targetNodeDefID).
		Find(&definitions).Error
	return definitions, err
}

// CreateBatchForWorkflow 为工作流批量创建连接线定义
func (r *lineDefinitionRepository) CreateBatchForWorkflow(ctx context.Context, workflowID uint, definitions []*models.LineDefinition) error {
	if len(definitions) == 0 {
		return nil
	}

	// 设置工作流ID
	for _, def := range definitions {
		def.WorkflowID = workflowID
	}

	return r.db.WithContext(ctx).CreateInBatches(definitions, 100).Error
}

// DeleteByWorkflowID 删除工作流的所有连接线定义
func (r *lineDefinitionRepository) DeleteByWorkflowID(ctx context.Context, workflowID uint) error {
	return r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Delete(&models.LineDefinition{}).Error
}
