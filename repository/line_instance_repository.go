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
	FindBySourceNodeInstID(ctx context.Context, sourceNodeInstID uint) ([]*models.LineInstance, error)
	FindByTargetNodeInstID(ctx context.Context, targetNodeInstID uint) ([]*models.LineInstance, error)
	FindByStatus(ctx context.Context, status models.LineInstanceStatus) ([]*models.LineInstance, error)

	// 状态管理
	UpdateStatus(ctx context.Context, id uint, status models.LineInstanceStatus) error
	Evaluate(ctx context.Context, id uint, result bool) error
	Skip(ctx context.Context, id uint) error
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

// FindBySourceNodeInstID 根据源节点实例ID查找连接线实例
func (r *lineInstanceRepository) FindBySourceNodeInstID(ctx context.Context, sourceNodeInstID uint) ([]*models.LineInstance, error) {
	var instances []*models.LineInstance
	err := r.db.WithContext(ctx).
		Where("source_node_inst_id = ?", sourceNodeInstID).
		Find(&instances).Error
	return instances, err
}

// FindByTargetNodeInstID 根据目标节点实例ID查找连接线实例
func (r *lineInstanceRepository) FindByTargetNodeInstID(ctx context.Context, targetNodeInstID uint) ([]*models.LineInstance, error) {
	var instances []*models.LineInstance
	err := r.db.WithContext(ctx).
		Where("target_node_inst_id = ?", targetNodeInstID).
		Find(&instances).Error
	return instances, err
}

// FindByStatus 根据状态查找连接线实例
func (r *lineInstanceRepository) FindByStatus(ctx context.Context, status models.LineInstanceStatus) ([]*models.LineInstance, error) {
	var instances []*models.LineInstance
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Find(&instances).Error
	return instances, err
}

// UpdateStatus 更新连接线实例状态
func (r *lineInstanceRepository) UpdateStatus(ctx context.Context, id uint, status models.LineInstanceStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.LineInstance{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Evaluate 评估连接线实例
func (r *lineInstanceRepository) Evaluate(ctx context.Context, id uint, result bool) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.LineInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":           models.LineInstanceStatusEvaluated,
			"condition_result": result,
			"evaluated_at":     now,
		}).Error
}

// Skip 跳过连接线实例
func (r *lineInstanceRepository) Skip(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.LineInstance{}).
		Where("id = ?", id).
		Update("status", models.LineInstanceStatusSkipped).Error
}
