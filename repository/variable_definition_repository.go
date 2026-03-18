/*
 * @module repository/variable_definition_repository
 * @description 变量定义Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> VariableDefinitionRepository -> GORM -> 数据库
 * @rules 实现变量定义的CRUD操作、工作流关联查询等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.6节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"

	"gorm.io/gorm"
)

// VariableDefinitionRepository 变量定义 Repository 接口
type VariableDefinitionRepository interface {
	BaseRepository[models.VariableDefinition]

	// 基础查询
	FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.VariableDefinition, error)
	FindByWorkflowIDAndKeyName(ctx context.Context, workflowID uint, keyName string) (*models.VariableDefinition, error)
	FindByScope(ctx context.Context, scope string) ([]*models.VariableDefinition, error)
	FindByNodeTemplateID(ctx context.Context, nodeTemplateID uint) ([]*models.VariableDefinition, error)

	// 批量操作
	CreateBatchForWorkflow(ctx context.Context, workflowID uint, definitions []*models.VariableDefinition) error
	CreateBatchForNodeTemplate(ctx context.Context, nodeTemplateID uint, definitions []*models.VariableDefinition) error
	DeleteByWorkflowID(ctx context.Context, workflowID uint) error
	DeleteByNodeTemplateID(ctx context.Context, nodeTemplateID uint) error
}

// variableDefinitionRepository 变量定义Repository实现
type variableDefinitionRepository struct {
	BaseRepository[models.VariableDefinition]
	db *gorm.DB
}

// NewVariableDefinitionRepository 创建VariableDefinition Repository实例
func NewVariableDefinitionRepository(db *gorm.DB) VariableDefinitionRepository {
	return &variableDefinitionRepository{
		BaseRepository: newBaseRepository[models.VariableDefinition](db),
		db:             db,
	}
}

// FindByWorkflowID 根据工作流ID查找变量定义
func (r *variableDefinitionRepository) FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.VariableDefinition, error) {
	var definitions []*models.VariableDefinition
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("id ASC").
		Find(&definitions).Error
	return definitions, err
}

// FindByWorkflowIDAndKeyName 根据工作流ID和key_name查找变量定义
func (r *variableDefinitionRepository) FindByWorkflowIDAndKeyName(ctx context.Context, workflowID uint, keyName string) (*models.VariableDefinition, error) {
	var definition models.VariableDefinition
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

// FindByScope 根据作用域查找变量定义
func (r *variableDefinitionRepository) FindByScope(ctx context.Context, scope string) ([]*models.VariableDefinition, error) {
	var definitions []*models.VariableDefinition
	err := r.db.WithContext(ctx).
		Where("scope = ?", scope).
		Find(&definitions).Error
	return definitions, err
}

// FindByNodeTemplateID 根据节点模板 ID 查找变量定义
func (r *variableDefinitionRepository) FindByNodeTemplateID(ctx context.Context, nodeTemplateID uint) ([]*models.VariableDefinition, error) {
	var definitions []*models.VariableDefinition
	err := r.db.WithContext(ctx).
		Where("node_template_id = ?", nodeTemplateID).
		Order("id ASC").
		Find(&definitions).Error
	return definitions, err
}

// CreateBatchForWorkflow 为工作流批量创建变量定义
func (r *variableDefinitionRepository) CreateBatchForWorkflow(ctx context.Context, workflowID uint, definitions []*models.VariableDefinition) error {
	if len(definitions) == 0 {
		return nil
	}

	// 设置工作流 ID
	for _, def := range definitions {
		def.WorkflowID = workflowID
	}

	return r.db.WithContext(ctx).CreateInBatches(definitions, 100).Error
}

// CreateBatchForNodeTemplate 为节点模板批量创建变量定义
func (r *variableDefinitionRepository) CreateBatchForNodeTemplate(ctx context.Context, nodeTemplateID uint, definitions []*models.VariableDefinition) error {
	if len(definitions) == 0 {
		return nil
	}

	// 设置节点模板 ID
	for _, def := range definitions {
		def.NodeTemplateID = &nodeTemplateID
	}

	return r.db.WithContext(ctx).CreateInBatches(definitions, 100).Error
}

// DeleteByWorkflowID 删除工作流的所有变量定义
func (r *variableDefinitionRepository) DeleteByWorkflowID(ctx context.Context, workflowID uint) error {
	return r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Delete(&models.VariableDefinition{}).Error
}

// DeleteByNodeTemplateID 删除节点模板的所有变量定义
func (r *variableDefinitionRepository) DeleteByNodeTemplateID(ctx context.Context, nodeTemplateID uint) error {
	return r.db.WithContext(ctx).
		Where("node_template_id = ?", nodeTemplateID).
		Delete(&models.VariableDefinition{}).Error
}
