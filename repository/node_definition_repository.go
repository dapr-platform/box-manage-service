/*
 * @module repository/node_definition_repository
 * @description 节点定义Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> NodeDefinitionRepository -> GORM -> 数据库
 * @rules 实现节点定义的CRUD操作、工作流关联查询等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.3节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"

	"gorm.io/gorm"
)

// NodeDefinitionRepository 节点定义Repository接口
type NodeDefinitionRepository interface {
	BaseRepository[models.NodeDefinition]

	// 基础查询
	FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.NodeDefinition, error)
	FindByWorkflowIDAndKeyName(ctx context.Context, workflowID uint, keyName string) (*models.NodeDefinition, error)
	FindByType(ctx context.Context, nodeType string) ([]*models.NodeDefinition, error)

	// 批量操作
	CreateBatchForWorkflow(ctx context.Context, workflowID uint, definitions []*models.NodeDefinition) error
	DeleteByWorkflowID(ctx context.Context, workflowID uint) error
}

// nodeDefinitionRepository 节点定义Repository实现
type nodeDefinitionRepository struct {
	BaseRepository[models.NodeDefinition]
	db *gorm.DB
}

// NewNodeDefinitionRepository 创建NodeDefinition Repository实例
func NewNodeDefinitionRepository(db *gorm.DB) NodeDefinitionRepository {
	return &nodeDefinitionRepository{
		BaseRepository: newBaseRepository[models.NodeDefinition](db),
		db:             db,
	}
}

// FindByWorkflowID 根据工作流ID查找节点定义
func (r *nodeDefinitionRepository) FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.NodeDefinition, error) {
	var definitions []*models.NodeDefinition
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("id ASC").
		Find(&definitions).Error
	return definitions, err
}

// FindByWorkflowIDAndKeyName 根据工作流ID和key_name查找节点定义
func (r *nodeDefinitionRepository) FindByWorkflowIDAndKeyName(ctx context.Context, workflowID uint, keyName string) (*models.NodeDefinition, error) {
	var definition models.NodeDefinition
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND key_name = ?", workflowID, keyName).
		First(&definition).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &definition, nil
}

// FindByType 根据类型查找节点定义
func (r *nodeDefinitionRepository) FindByType(ctx context.Context, nodeType string) ([]*models.NodeDefinition, error) {
	var definitions []*models.NodeDefinition
	err := r.db.WithContext(ctx).
		Where("type = ?", nodeType).
		Find(&definitions).Error
	return definitions, err
}

// CreateBatchForWorkflow 为工作流批量创建节点定义
func (r *nodeDefinitionRepository) CreateBatchForWorkflow(ctx context.Context, workflowID uint, definitions []*models.NodeDefinition) error {
	if len(definitions) == 0 {
		return nil
	}

	// 设置工作流ID
	for _, def := range definitions {
		def.WorkflowID = workflowID
	}

	return r.db.WithContext(ctx).CreateInBatches(definitions, 100).Error
}

// DeleteByWorkflowID 删除工作流的所有节点定义
func (r *nodeDefinitionRepository) DeleteByWorkflowID(ctx context.Context, workflowID uint) error {
	return r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Delete(&models.NodeDefinition{}).Error
}
