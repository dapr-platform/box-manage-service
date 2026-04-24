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
	FindByDeploymentID(ctx context.Context, deploymentID uint) ([]*models.WorkflowSchedule, error)
	FindByType(ctx context.Context, scheduleType models.ScheduleType) ([]*models.WorkflowSchedule, error)
	FindEnabled(ctx context.Context) ([]*models.WorkflowSchedule, error)
	FindDisabled(ctx context.Context) ([]*models.WorkflowSchedule, error)

	// 过滤分页查询
	FindByFilter(ctx context.Context, filter *ScheduleFilter) ([]*models.WorkflowSchedule, int64, error)

	// 调度查询
	FindDueSchedules(ctx context.Context, now time.Time) ([]*models.WorkflowSchedule, error)

	// 状态管理
	Enable(ctx context.Context, id uint) error
	Disable(ctx context.Context, id uint) error

	// 执行管理
	UpdateLastRunTime(ctx context.Context, id uint, runTime time.Time) error
	UpdateNextRunTime(ctx context.Context, id uint, nextTime time.Time) error
	IncrementRunCount(ctx context.Context, id uint) error

	// 统计
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
}

// ScheduleFilter 调度配置过滤条件
type ScheduleFilter struct {
	WorkflowID   uint   // 精确匹配
	DeploymentID uint   // 精确匹配
	Name         string // 模糊匹配
	Page         int
	PageSize     int
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

// FindByDeploymentID 根据部署ID查找调度配置
func (r *workflowScheduleRepository) FindByDeploymentID(ctx context.Context, deploymentID uint) ([]*models.WorkflowSchedule, error) {
	var schedules []*models.WorkflowSchedule
	err := r.db.WithContext(ctx).
		Where("deployment_id = ?", deploymentID).
		Order("created_at DESC").
		Find(&schedules).Error
	return schedules, err
}

// FindByType 根据调度类型查找调度配置
func (r *workflowScheduleRepository) FindByType(ctx context.Context, scheduleType models.ScheduleType) ([]*models.WorkflowSchedule, error) {
	var schedules []*models.WorkflowSchedule
	err := r.db.WithContext(ctx).
		Where("schedule_type = ?", scheduleType).
		Find(&schedules).Error
	return schedules, err
}

// FindEnabled 查找启用的调度配置
func (r *workflowScheduleRepository) FindEnabled(ctx context.Context) ([]*models.WorkflowSchedule, error) {
	var schedules []*models.WorkflowSchedule
	err := r.db.WithContext(ctx).
		Where("is_enabled = ?", true).
		Find(&schedules).Error
	return schedules, err
}

// FindDisabled 查找禁用的调度配置
func (r *workflowScheduleRepository) FindDisabled(ctx context.Context) ([]*models.WorkflowSchedule, error) {
	var schedules []*models.WorkflowSchedule
	err := r.db.WithContext(ctx).
		Where("is_enabled = ?", false).
		Find(&schedules).Error
	return schedules, err
}

// FindDueSchedules 查找到期的调度配置
func (r *workflowScheduleRepository) FindDueSchedules(ctx context.Context, now time.Time) ([]*models.WorkflowSchedule, error) {
	var schedules []*models.WorkflowSchedule
	err := r.db.WithContext(ctx).
		Where("is_enabled = ? AND schedule_type = ? AND (next_run_time IS NULL OR next_run_time <= ?)",
			true, models.ScheduleTypeCron, now).
		Find(&schedules).Error
	return schedules, err
}

// Enable 启用调度配置
func (r *workflowScheduleRepository) Enable(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Where("id = ?", id).
		Update("is_enabled", true).Error
}

// Disable 禁用调度配置
func (r *workflowScheduleRepository) Disable(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Where("id = ?", id).
		Update("is_enabled", false).Error
}

// UpdateLastRunTime 更新最后执行时间
func (r *workflowScheduleRepository) UpdateLastRunTime(ctx context.Context, id uint, runTime time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Where("id = ?", id).
		Update("last_run_time", runTime).Error
}

// UpdateNextRunTime 更新下次执行时间
func (r *workflowScheduleRepository) UpdateNextRunTime(ctx context.Context, id uint, nextTime time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Where("id = ?", id).
		Update("next_run_time", nextTime).Error
}

// IncrementRunCount 增加执行次数
func (r *workflowScheduleRepository) IncrementRunCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Where("id = ?", id).
		UpdateColumn("run_count", gorm.Expr("run_count + 1")).Error
}

// FindByFilter 过滤分页查询
func (r *workflowScheduleRepository) FindByFilter(ctx context.Context, filter *ScheduleFilter) ([]*models.WorkflowSchedule, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.WorkflowSchedule{})

	if filter.WorkflowID > 0 {
		q = q.Where("workflow_id = ?", filter.WorkflowID)
	}
	if filter.DeploymentID > 0 {
		q = q.Where("deployment_id = ?", filter.DeploymentID)
	}
	if filter.Name != "" {
		q = q.Where("name ILIKE ?", "%"+filter.Name+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	var schedules []*models.WorkflowSchedule
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&schedules).Error
	return schedules, total, err
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
		Where("is_enabled = ?", true).
		Count(&enabled).Error; err != nil {
		return nil, err
	}
	stats["enabled"] = enabled

	// 按类型统计
	type typeCount struct {
		ScheduleType models.ScheduleType `gorm:"column:schedule_type"`
		Count        int64
	}
	var typeCounts []typeCount
	if err := r.db.WithContext(ctx).
		Model(&models.WorkflowSchedule{}).
		Select("schedule_type, count(*) as count").
		Group("schedule_type").
		Find(&typeCounts).Error; err != nil {
		return nil, err
	}

	typeStats := make(map[string]int64)
	for _, tc := range typeCounts {
		typeStats[string(tc.ScheduleType)] = tc.Count
	}
	stats["by_type"] = typeStats

	return stats, nil
}
