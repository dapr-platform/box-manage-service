/*
 * @module repository/variable_instance_repository
 * @description 变量实例Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> VariableInstanceRepository -> GORM -> 数据库
 * @rules 实现变量实例的CRUD操作、值管理等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.7节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"

	"gorm.io/gorm"
)

// VariableInstanceRepository 变量实例Repository接口
type VariableInstanceRepository interface {
	BaseRepository[models.VariableInstance]

	// 基础查询
	FindByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint) ([]*models.VariableInstance, error)
	FindByWorkflowInstanceIDAndKeyName(ctx context.Context, workflowInstanceID uint, keyName string) (*models.VariableInstance, error)
	FindByScope(ctx context.Context, scope string) ([]*models.VariableInstance, error)

	// 值管理
	UpdateValue(ctx context.Context, id uint, value interface{}) error
	UpdateValueByKeyName(ctx context.Context, workflowInstanceID uint, keyName string, value interface{}) error
}

// variableInstanceRepository 变量实例Repository实现
type variableInstanceRepository struct {
	BaseRepository[models.VariableInstance]
	db *gorm.DB
}

// NewVariableInstanceRepository 创建VariableInstance Repository实例
func NewVariableInstanceRepository(db *gorm.DB) VariableInstanceRepository {
	return &variableInstanceRepository{
		BaseRepository: newBaseRepository[models.VariableInstance](db),
		db:             db,
	}
}

// FindByWorkflowInstanceID 根据工作流实例ID查找变量实例
func (r *variableInstanceRepository) FindByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint) ([]*models.VariableInstance, error) {
	var instances []*models.VariableInstance
	err := r.db.WithContext(ctx).
		Where("workflow_instance_id = ?", workflowInstanceID).
		Order("id ASC").
		Find(&instances).Error
	return instances, err
}

// FindByWorkflowInstanceIDAndKeyName 根据工作流实例ID和key_name查找变量实例
func (r *variableInstanceRepository) FindByWorkflowInstanceIDAndKeyName(ctx context.Context, workflowInstanceID uint, keyName string) (*models.VariableInstance, error) {
	var instance models.VariableInstance
	err := r.db.WithContext(ctx).
		Where("workflow_instance_id = ? AND key_name = ?", workflowInstanceID, keyName).
		First(&instance).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &instance, nil
}

// FindByScope 根据作用域查找变量实例
func (r *variableInstanceRepository) FindByScope(ctx context.Context, scope string) ([]*models.VariableInstance, error) {
	var instances []*models.VariableInstance
	err := r.db.WithContext(ctx).
		Where("scope = ?", scope).
		Find(&instances).Error
	return instances, err
}

// UpdateValue 更新变量实例的值
func (r *variableInstanceRepository) UpdateValue(ctx context.Context, id uint, value interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.VariableInstance{}).
		Where("id = ?", id).
		Update("value", models.ValueJSON{Data: value}).Error
}

// UpdateValueByKeyName 根据key_name更新变量实例的值
func (r *variableInstanceRepository) UpdateValueByKeyName(ctx context.Context, workflowInstanceID uint, keyName string, value interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.VariableInstance{}).
		Where("workflow_instance_id = ? AND key_name = ?", workflowInstanceID, keyName).
		Update("value", models.ValueJSON{Data: value}).Error
}
