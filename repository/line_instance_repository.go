/*
 * @module repository/line_instance_repository
 * @description 连接线实例Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> LineInstanceRepository -> GORM -> 数据库
 * @rules 实现连接线实例的CRUD操作、状态管理等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.8节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// LineInstanceRepository 连接线实例Repository接口
type LineInstanceRepository interface {
	BaseRepository[models.LineInstance]

	// 基础查询
	FindByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint) ([]*models.LineInstance, error)
	FindByLineID(ctx context.Context, workflowInstanceID uint, lineID string) (*models.LineInstance, error)
	FindBySourceNodeID(ctx context.Context, workflowInstanceID uint, sourceNodeID string) ([]*models.LineInstance, error)
	FindByTargetNodeID(ctx context.Context, workflowInstanceID uint, targetNodeID string) ([]*models.LineInstance, error)
	FindByExecuted(ctx context.Context, workflowInstanceID uint, executed bool) ([]*models.LineInstance, error)

	// 状态管理
	MarkAsExecuted(ctx context.Context, id uint) error
	Evaluate(ctx context.Context, id uint, result bool, context string) error
	UpdateError(ctx context.Context, id uint, errorMsg string) error
}

// lineInstanceRepository 连接线实例Repository实现
type lineInstanceRepository struct {
	BaseRepository[models.LineInstance]
	db *gorm.DB
}

// NewLineInstanceRepository 创建LineInstance Repository实例
func NewLineInstanceRepository(db *gorm.DB) LineInstanceRepository {
	return &lineInstanceRepository{
		BaseRepository: newBaseRepository[models.LineInstance](db),
		db:             db,
	}
}

// FindByWorkflowInstanceID 根据工作流实例ID查找连接线实例
func (r *lineInstanceRepository) FindByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint) ([]*models.LineInstance, error) {
	var instances []*models.LineInstance
	err := r.db.WithContext(ctx).
		Where("workflow_instance_id = ?", workflowInstanceID).
		Order("id ASC").
		Find(&instances).Error
	return instances, err
}

// FindByLineID 根据连接线ID查找连接线实例
func (r *lineInstanceRepository) FindByLineID(ctx context.Context, workflowInstanceID uint, lineID string) (*models.LineInstance, error) {
	var instance models.LineInstance
	err := r.db.WithContext(ctx).
		Where("workflow_instance_id = ? AND line_id = ?", workflowInstanceID, lineID).
		First(&instance).Error
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// FindBySourceNodeID 根据源节点ID查找连接线实例
func (r *lineInstanceRepository) FindBySourceNodeID(ctx context.Context, workflowInstanceID uint, sourceNodeID string) ([]*models.LineInstance, error) {
	var instances []*models.LineInstance
	err := r.db.WithContext(ctx).
		Where("workflow_instance_id = ? AND source_node_id = ?", workflowInstanceID, sourceNodeID).
		Find(&instances).Error
	return instances, err
}

// FindByTargetNodeID 根据目标节点ID查找连接线实例
func (r *lineInstanceRepository) FindByTargetNodeID(ctx context.Context, workflowInstanceID uint, targetNodeID string) ([]*models.LineInstance, error) {
	var instances []*models.LineInstance
	err := r.db.WithContext(ctx).
		Where("workflow_instance_id = ? AND target_node_id = ?", workflowInstanceID, targetNodeID).
		Find(&instances).Error
	return instances, err
}

// FindByExecuted 根据执行状态查找连接线实例
func (r *lineInstanceRepository) FindByExecuted(ctx context.Context, workflowInstanceID uint, executed bool) ([]*models.LineInstance, error) {
	var instances []*models.LineInstance
	err := r.db.WithContext(ctx).
		Where("workflow_instance_id = ? AND executed = ?", workflowInstanceID, executed).
		Find(&instances).Error
	return instances, err
}

// MarkAsExecuted 标记连接线实例为已执行
func (r *lineInstanceRepository) MarkAsExecuted(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.LineInstance{}).
		Where("id = ?", id).
		Update("executed", true).Error
}

// Evaluate 评估连接线实例
func (r *lineInstanceRepository) Evaluate(ctx context.Context, id uint, result bool, context string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.LineInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"condition_result":  result,
			"condition_context": context,
			"evaluated_at":      now,
			"executed":          true,
		}).Error
}

// UpdateError 更新连接线实例错误信息
func (r *lineInstanceRepository) UpdateError(ctx context.Context, id uint, errorMsg string) error {
	return r.db.WithContext(ctx).
		Model(&models.LineInstance{}).
		Where("id = ?", id).
		Update("error_message", errorMsg).Error
}
