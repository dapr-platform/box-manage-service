/*
 * @module repository/workflow_instance_repository
 * @description 工作流实例Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> WorkflowInstanceRepository -> GORM -> 数据库
 * @rules 实现工作流实例的CRUD操作、状态管理、统计查询等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.4节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// WorkflowInstanceRepository 工作流实例Repository接口
type WorkflowInstanceRepository interface {
	BaseRepository[models.WorkflowInstance]

	// 基础查询
	FindByInstanceID(ctx context.Context, instanceID string) (*models.WorkflowInstance, error)
	FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowInstance, error)
	FindByBoxID(ctx context.Context, boxID uint) ([]*models.WorkflowInstance, error)
	FindByStatus(ctx context.Context, status models.WorkflowInstanceStatus) ([]*models.WorkflowInstance, error)
	FindRunning(ctx context.Context) ([]*models.WorkflowInstance, error)

	// 状态管理
	UpdateStatus(ctx context.Context, id uint, status models.WorkflowInstanceStatus) error
	UpdateProgress(ctx context.Context, id uint, progress float64) error
	Start(ctx context.Context, id uint) error
	Complete(ctx context.Context, id uint) error
	Fail(ctx context.Context, id uint, errorMsg string) error
	Cancel(ctx context.Context, id uint) error

	// 时间管理
	UpdateStartTime(ctx context.Context, id uint, startTime time.Time) error
	UpdateEndTime(ctx context.Context, id uint, endTime time.Time) error

	// 统计查询
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
	GetStatusDistribution(ctx context.Context) (map[models.WorkflowInstanceStatus]int64, error)
	GetExecutionReport(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error)

	// 关联加载
	LoadWithInstances(ctx context.Context, instance *models.WorkflowInstance) error
}

// workflowInstanceRepository 工作流实例Repository实现
type workflowInstanceRepository struct {
	BaseRepository[models.WorkflowInstance]
	db *gorm.DB
}

// NewWorkflowInstanceRepository 创建WorkflowInstance Repository实例
func NewWorkflowInstanceRepository(db *gorm.DB) WorkflowInstanceRepository {
	return &workflowInstanceRepository{
		BaseRepository: newBaseRepository[models.WorkflowInstance](db),
		db:             db,
	}
}

// FindByInstanceID 根据实例ID查找实例
func (r *workflowInstanceRepository) FindByInstanceID(ctx context.Context, instanceID string) (*models.WorkflowInstance, error) {
	var instance models.WorkflowInstance
	err := r.db.WithContext(ctx).
		Where("instance_id = ?", instanceID).
		First(&instance).Error
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

// FindByWorkflowID 根据工作流ID查找实例
func (r *workflowInstanceRepository) FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowInstance, error) {
	var instances []*models.WorkflowInstance
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("created_at DESC").
		Find(&instances).Error
	return instances, err
}

// FindByBoxID 根据盒子ID查找实例
func (r *workflowInstanceRepository) FindByBoxID(ctx context.Context, boxID uint) ([]*models.WorkflowInstance, error) {
	var instances []*models.WorkflowInstance
	err := r.db.WithContext(ctx).
		Where("box_id = ?", boxID).
		Order("created_at DESC").
		Find(&instances).Error
	return instances, err
}

// FindByStatus 根据状态查找实例
func (r *workflowInstanceRepository) FindByStatus(ctx context.Context, status models.WorkflowInstanceStatus) ([]*models.WorkflowInstance, error) {
	var instances []*models.WorkflowInstance
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at DESC").
		Find(&instances).Error
	return instances, err
}

// FindRunning 查找运行中的实例
func (r *workflowInstanceRepository) FindRunning(ctx context.Context) ([]*models.WorkflowInstance, error) {
	return r.FindByStatus(ctx, models.WorkflowInstanceStatusRunning)
}

// UpdateStatus 更新实例状态
func (r *workflowInstanceRepository) UpdateStatus(ctx context.Context, id uint, status models.WorkflowInstanceStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateProgress 更新实例进度
func (r *workflowInstanceRepository) UpdateProgress(ctx context.Context, id uint, progress float64) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Where("id = ?", id).
		Update("progress", progress).Error
}

// Start 启动实例
func (r *workflowInstanceRepository) Start(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     models.WorkflowInstanceStatusRunning,
			"started_at": now,
		}).Error
}

// Complete 完成实例
func (r *workflowInstanceRepository) Complete(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":   models.WorkflowInstanceStatusCompleted,
			"ended_at": now,
			"progress": 100.0,
		}).Error
}

// Fail 失败实例
func (r *workflowInstanceRepository) Fail(ctx context.Context, id uint, errorMsg string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        models.WorkflowInstanceStatusFailed,
			"ended_at":      now,
			"error_message": errorMsg,
		}).Error
}

// Cancel 取消实例
func (r *workflowInstanceRepository) Cancel(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":   models.WorkflowInstanceStatusCancelled,
			"ended_at": now,
		}).Error
}

// UpdateStartTime 更新开始时间
func (r *workflowInstanceRepository) UpdateStartTime(ctx context.Context, id uint, startTime time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Where("id = ?", id).
		Update("started_at", startTime).Error
}

// UpdateEndTime 更新结束时间
func (r *workflowInstanceRepository) UpdateEndTime(ctx context.Context, id uint, endTime time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Where("id = ?", id).
		Update("ended_at", endTime).Error
}

// GetStatistics 获取工作流实例统计信息
func (r *workflowInstanceRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总实例数
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowInstance{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 运行中实例数
	var running int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowInstance{}).
		Where("status = ?", models.WorkflowInstanceStatusRunning).
		Count(&running).Error; err != nil {
		return nil, err
	}
	stats["running"] = running

	// 已完成实例数
	var completed int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowInstance{}).
		Where("status = ?", models.WorkflowInstanceStatusCompleted).
		Count(&completed).Error; err != nil {
		return nil, err
	}
	stats["completed"] = completed

	// 失败实例数
	var failed int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowInstance{}).
		Where("status = ?", models.WorkflowInstanceStatusFailed).
		Count(&failed).Error; err != nil {
		return nil, err
	}
	stats["failed"] = failed

	return stats, nil
}

// GetStatusDistribution 获取实例状态分布
func (r *workflowInstanceRepository) GetStatusDistribution(ctx context.Context) (map[models.WorkflowInstanceStatus]int64, error) {
	distribution := make(map[models.WorkflowInstanceStatus]int64)

	type statusCount struct {
		Status models.WorkflowInstanceStatus
		Count  int64
	}

	var results []statusCount
	err := r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	for _, result := range results {
		distribution[result.Status] = result.Count
	}

	return distribution, nil
}

// GetExecutionReport 获取执行报告
func (r *workflowInstanceRepository) GetExecutionReport(ctx context.Context, startDate, endDate time.Time) (map[string]interface{}, error) {
	report := make(map[string]interface{})

	// 时间范围内的总执行数
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowInstance{}).
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Count(&total).Error; err != nil {
		return nil, err
	}
	report["total"] = total

	// 成功率
	var completed int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowInstance{}).
		Where("created_at BETWEEN ? AND ? AND status = ?", startDate, endDate, models.WorkflowInstanceStatusCompleted).
		Count(&completed).Error; err != nil {
		return nil, err
	}
	report["completed"] = completed

	if total > 0 {
		report["success_rate"] = float64(completed) / float64(total) * 100
	} else {
		report["success_rate"] = 0.0
	}

	// 平均执行时间
	type avgDuration struct {
		AvgSeconds float64
	}
	var avg avgDuration
	if err := r.db.WithContext(ctx).
		Model(&models.WorkflowInstance{}).
		Where("created_at BETWEEN ? AND ? AND started_at IS NOT NULL AND ended_at IS NOT NULL", startDate, endDate).
		Select("AVG(EXTRACT(EPOCH FROM (ended_at - started_at))) as avg_seconds").
		Scan(&avg).Error; err != nil {
		return nil, err
	}
	report["avg_duration_seconds"] = avg.AvgSeconds

	return report, nil
}

// LoadWithInstances 加载实例及其关联的节点实例、变量实例等
func (r *workflowInstanceRepository) LoadWithInstances(ctx context.Context, instance *models.WorkflowInstance) error {
	return r.db.WithContext(ctx).
		Preload("NodeInstances").
		Preload("VariableInstances").
		Preload("LineInstances").
		First(instance, instance.ID).Error
}
