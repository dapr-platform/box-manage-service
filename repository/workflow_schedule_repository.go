/*
 * @module repository/workflow_schedule_repository
 * @description 工作流调度配置Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> WorkflowScheduleRepository -> GORM -> 数据库
 * @rules 实现工作流调度配置的CRUD操作、状态管理等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.10节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// WorkflowScheduleRepository 工作流调度配置Repository接口
type WorkflowScheduleRepository interface {
	BaseRepository[models.WorkflowSchedule]

	// 基础查询
	FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowSchedule, error)
	FindByType(ctx context.Context, scheduleType models.ScheduleType) ([]*models.WorkflowSchedule, error)
	FindByStatus(ctx context.Context, status models.WorkflowScheduleStatus) ([]*models.WorkflowSchedule, error)
	FindEnabled(ctx context.Context) ([]*models.WorkflowSchedule, error)

	// 调度查询
	FindDueSchedules(ctx context.Context, now time.Time) ([]*models.WorkflowSchedule, error)

	// 状态管理
	UpdateStatus(ctx context.Context, id uint, status models.WorkflowScheduleStatus) error
	Enable(ctx context.Context, id uint) error
	Disable(ctx context.Context, id uint) error

	// 执行管理
	UpdateLastExecutedAt(ctx context.Context, id uint, executedAt time.Time) error
	UpdateNextExecutionAt(ctx context.Context, id uint, nextAt time.Time) error
	IncrementExecutionCount(ctx context.Context, id uint) error

	// 统计
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
}

// workflowScheduleRepository 工作流调度配置Repository实现
type workflowScheduleRepository struct {
	BaseRepository[models.WorkflowSchedule]
	db *gorm.DB
}

// NewWorkflowScheduleRepository 创建WorkflowSchedule Repository实例
func NewWorkflowScheduleRepository(db *gorm.DB) WorkflowScheduleRepository {
	return &workflowScheduleRepository{
		BaseRepository: newBaseRepository[models.WorkflowSchedule](db),
		db:             db,
	}
}

// FindByWorkflowID 根据工作流ID查找调度配置
func (r *workflowScheduleRepository) FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowSchedule, error) {
	var schedules []*models.WorkflowSchedule
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("created_at DESC").
		Find(&schedules).Error
	return schedules, err
}

// FindByType 根据调度类型查找调度配置
func (r *workflowScheduleRepository) FindByType(ctx context.Context, scheduleType models.ScheduleType) ([]*models.WorkflowSchedule, error) {
	var schedules []*models.WorkflowSchedule
	err := r.db.WithContext(ctx).
		Where("type = ?", scheduleType).
		Find(&schedules).Error
	return schedules, err
}

// FindByStatus 根据状态查找调度配置
func (r *workflowScheduleRepository) FindByStatus(ctx context.Context, status models.WorkflowScheduleStatus) ([]*models.WorkflowSchedule, error) {
	var schedules []*models.WorkflowSchedule
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Find(&schedules).Error
	return schedules, err
}

// FindEnabled 查找启用的调度配置
func (r *workflowScheduleRepository) FindEnabled(ctx context.Context) ([]*models.WorkflowSchedule, error) {
	return r.FindByStatus(ctx, models.WorkflowScheduleStatusEnabled)
}

// FindDueSchedules 查找到期的调度配置
func (r *workflowScheduleRepository) FindDueSchedules(ctx context.Context, now time.Time) ([]*models.WorkflowSchedule, error) {
	var schedules []*models.WorkflowSchedule
	err := r.db.WithContext(ctx).
		Where("status = ? AND type = ? AND (next_execution_at IS NULL OR next_execution_at <= ?)",
			models.WorkflowScheduleStatusEnabled, models.ScheduleTypeCron, now).
		Find(&schedules).Error
	return schedules, err
}

// UpdateStatus 更新调度配置状态
func (r *workflowScheduleRepository) UpdateStatus(ctx context.Context, id uint, status models.WorkflowScheduleStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Enable 启用调度配置
func (r *workflowScheduleRepository) Enable(ctx context.Context, id uint) error {
	return r.UpdateStatus(ctx, id, models.WorkflowScheduleStatusEnabled)
}

// Disable 禁用调度配置
func (r *workflowScheduleRepository) Disable(ctx context.Context, id uint) error {
	return r.UpdateStatus(ctx, id, models.WorkflowScheduleStatusDisabled)
}

// UpdateLastExecutedAt 更新最后执行时间
func (r *workflowScheduleRepository) UpdateLastExecutedAt(ctx context.Context, id uint, executedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Where("id = ?", id).
		Update("last_executed_at", executedAt).Error
}

// UpdateNextExecutionAt 更新下次执行时间
func (r *workflowScheduleRepository) UpdateNextExecutionAt(ctx context.Context, id uint, nextAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Where("id = ?", id).
		Update("next_execution_at", nextAt).Error
}

// IncrementExecutionCount 增加执行次数
func (r *workflowScheduleRepository) IncrementExecutionCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Where("id = ?", id).
		UpdateColumn("execution_count", gorm.Expr("execution_count + 1")).Error
}

// GetStatistics 获取调度配置统计信息
func (r *workflowScheduleRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总调度配置数
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowSchedule{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 启用的调度配置数
	var enabled int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowSchedule{}).
		Where("status = ?", models.WorkflowScheduleStatusEnabled).
		Count(&enabled).Error; err != nil {
		return nil, err
	}
	stats["enabled"] = enabled

	// 按类型统计
	type typeCount struct {
		Type  models.ScheduleType
		Count int64
	}
	var typeCounts []typeCount
	if err := r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Select("type, count(*) as count").
		Group("type").
		Find(&typeCounts).Error; err != nil {
		return nil, err
	}

	typeStats := make(map[string]int64)
	for _, tc := range typeCounts {
		typeStats[string(tc.Type)] = tc.Count
	}
	stats["by_type"] = typeStats

	return stats, nil
}
