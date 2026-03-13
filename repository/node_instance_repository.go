/*
 * @module repository/node_instance_repository
 * @description 节点实例Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> NodeInstanceRepository -> GORM -> 数据库
 * @rules 实现节点实例的CRUD操作、状态管理等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.5节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// NodeInstanceRepository 节点实例Repository接口
type NodeInstanceRepository interface {
	BaseRepository[models.NodeInstance]

	// 基础查询
	FindByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint) ([]*models.NodeInstance, error)
	FindByStatus(ctx context.Context, status models.NodeInstanceStatus) ([]*models.NodeInstance, error)
	FindRunning(ctx context.Context) ([]*models.NodeInstance, error)

	// 状态管理
	UpdateStatus(ctx context.Context, id uint, status models.NodeInstanceStatus) error
	Start(ctx context.Context, id uint) error
	Complete(ctx context.Context, id uint, output map[string]interface{}) error
	Fail(ctx context.Context, id uint, errorMsg string) error
	Skip(ctx context.Context, id uint) error

	// 输出管理
	UpdateOutput(ctx context.Context, id uint, output map[string]interface{}) error

	// 时间管理
	UpdateStartTime(ctx context.Context, id uint, startTime time.Time) error
	UpdateEndTime(ctx context.Context, id uint, endTime time.Time) error
}

// nodeInstanceRepository 节点实例Repository实现
type nodeInstanceRepository struct {
	BaseRepository[models.NodeInstance]
	db *gorm.DB
}

// NewNodeInstanceRepository 创建NodeInstance Repository实例
func NewNodeInstanceRepository(db *gorm.DB) NodeInstanceRepository {
	return &nodeInstanceRepository{
		BaseRepository: newBaseRepository[models.NodeInstance](db),
		db:             db,
	}
}

// FindByWorkflowInstanceID 根据工作流实例ID查找节点实例
func (r *nodeInstanceRepository) FindByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint) ([]*models.NodeInstance, error) {
	var instances []*models.NodeInstance
	err := r.db.WithContext(ctx).
		Where("workflow_instance_id = ?", workflowInstanceID).
		Order("id ASC").
		Find(&instances).Error
	return instances, err
}

// FindByStatus 根据状态查找节点实例
func (r *nodeInstanceRepository) FindByStatus(ctx context.Context, status models.NodeInstanceStatus) ([]*models.NodeInstance, error) {
	var instances []*models.NodeInstance
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Find(&instances).Error
	return instances, err
}

// FindRunning 查找运行中的节点实例
func (r *nodeInstanceRepository) FindRunning(ctx context.Context) ([]*models.NodeInstance, error) {
	return r.FindByStatus(ctx, models.NodeInstanceStatusRunning)
}

// UpdateStatus 更新节点实例状态
func (r *nodeInstanceRepository) UpdateStatus(ctx context.Context, id uint, status models.NodeInstanceStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.NodeInstance{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Start 启动节点实例
func (r *nodeInstanceRepository) Start(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.NodeInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     models.NodeInstanceStatusRunning,
			"started_at": now,
		}).Error
}

// Complete 完成节点实例
func (r *nodeInstanceRepository) Complete(ctx context.Context, id uint, output map[string]interface{}) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.NodeInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      models.NodeInstanceStatusCompleted,
			"ended_at":    now,
			"output_data": output,
		}).Error
}

// Fail 失败节点实例
func (r *nodeInstanceRepository) Fail(ctx context.Context, id uint, errorMsg string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.NodeInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        models.NodeInstanceStatusFailed,
			"ended_at":      now,
			"error_message": errorMsg,
		}).Error
}

// Skip 跳过节点实例
func (r *nodeInstanceRepository) Skip(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.NodeInstance{}).
		Where("id = ?", id).
		Update("status", models.NodeInstanceStatusSkipped).Error
}

// UpdateOutput 更新节点实例输出
func (r *nodeInstanceRepository) UpdateOutput(ctx context.Context, id uint, output map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&models.NodeInstance{}).
		Where("id = ?", id).
		Update("output_data", output).Error
}

// UpdateStartTime 更新开始时间
func (r *nodeInstanceRepository) UpdateStartTime(ctx context.Context, id uint, startTime time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.NodeInstance{}).
		Where("id = ?", id).
		Update("started_at", startTime).Error
}

// UpdateEndTime 更新结束时间
func (r *nodeInstanceRepository) UpdateEndTime(ctx context.Context, id uint, endTime time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.NodeInstance{}).
		Where("id = ?", id).
		Update("ended_at", endTime).Error
}
