package repository

import (
	"box-manage-service/models"
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// WorkflowScheduleInstanceRepository 调度实例仓储接口
type WorkflowScheduleInstanceRepository interface {
	BaseRepository[models.WorkflowScheduleInstance]

	// FindByScheduleID 根据调度ID查询实例列表
	FindByScheduleID(ctx context.Context, scheduleID uint, page, pageSize int) ([]*models.WorkflowScheduleInstance, int64, error)

	// FindByInstanceID 根据实例ID查询
	FindByInstanceID(ctx context.Context, instanceID string) (*models.WorkflowScheduleInstance, error)

	// FindByFilter 根据过滤条件查询
	FindByFilter(ctx context.Context, filter *ScheduleInstanceFilter) ([]*models.WorkflowScheduleInstance, int64, error)

	// UpdateStatus 更新状态
	UpdateStatus(ctx context.Context, instanceID string, status string) error

	// UpdateWorkflowInstanceIDs 更新工作流实例ID列表
	UpdateWorkflowInstanceIDs(ctx context.Context, instanceID string, workflowInstanceIDs interface{}) error

	// UpdateCounts 更新成功/失败计数
	UpdateCounts(ctx context.Context, instanceID string, successCount, failedCount int) error

	// GetStatistics 获取统计数据
	GetStatistics(ctx context.Context, scheduleID uint, startDate, endDate *time.Time) (*ScheduleInstanceStatistics, error)

	// BatchDelete 批量删除
	BatchDelete(ctx context.Context, instanceIDs []string) (int64, error)
}

// ScheduleInstanceFilter 调度实例过滤器
type ScheduleInstanceFilter struct {
	ScheduleID  *uint
	Status      *string
	TriggerType *string
	StartDate   *time.Time
	EndDate     *time.Time
	Page        int
	PageSize    int
}

// ScheduleInstanceStatistics 调度实例统计
type ScheduleInstanceStatistics struct {
	ScheduleID              uint
	ScheduleName            string
	TotalInstances          int
	CompletedInstances      int
	FailedInstances         int
	CancelledInstances      int
	SuccessRate             float64
	AvgDuration             int
	TotalWorkflowInstances  int
	WorkflowSuccessCount    int
	WorkflowFailedCount     int
	WorkflowSuccessRate     float64
	TriggerTypeDistribution map[string]int
	DailyStatistics         []DailyStatistic
}

// DailyStatistic 每日统计
type DailyStatistic struct {
	Date        string
	Total       int
	Completed   int
	Failed      int
	SuccessRate float64
}

type workflowScheduleInstanceRepository struct {
	BaseRepository[models.WorkflowScheduleInstance]
	db *gorm.DB
}

// NewWorkflowScheduleInstanceRepository 创建调度实例仓储
func NewWorkflowScheduleInstanceRepository(db *gorm.DB) WorkflowScheduleInstanceRepository {
	return &workflowScheduleInstanceRepository{
		BaseRepository: newBaseRepository[models.WorkflowScheduleInstance](db),
		db:             db,
	}
}

// FindByScheduleID 根据调度ID查询实例列表
func (r *workflowScheduleInstanceRepository) FindByScheduleID(ctx context.Context, scheduleID uint, page, pageSize int) ([]*models.WorkflowScheduleInstance, int64, error) {
	var instances []*models.WorkflowScheduleInstance
	var total int64

	query := r.db.WithContext(ctx).Model(&models.WorkflowScheduleInstance{}).
		Where("schedule_id = ?", scheduleID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&instances).Error; err != nil {
		return nil, 0, err
	}

	return instances, total, nil
}

// FindByInstanceID 根据实例ID查询
func (r *workflowScheduleInstanceRepository) FindByInstanceID(ctx context.Context, instanceID string) (*models.WorkflowScheduleInstance, error) {
	var instance models.WorkflowScheduleInstance
	if err := r.db.WithContext(ctx).
		Where("instance_id = ?", instanceID).
		First(&instance).Error; err != nil {
		return nil, err
	}
	return &instance, nil
}

// FindByFilter 根据过滤条件查询
func (r *workflowScheduleInstanceRepository) FindByFilter(ctx context.Context, filter *ScheduleInstanceFilter) ([]*models.WorkflowScheduleInstance, int64, error) {
	var instances []*models.WorkflowScheduleInstance
	var total int64

	query := r.db.WithContext(ctx).Model(&models.WorkflowScheduleInstance{})

	// 应用过滤条件
	if filter.ScheduleID != nil {
		query = query.Where("schedule_id = ?", *filter.ScheduleID)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.TriggerType != nil {
		query = query.Where("trigger_type = ?", *filter.TriggerType)
	}
	if filter.StartDate != nil {
		query = query.Where("trigger_time >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("trigger_time <= ?", *filter.EndDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("trigger_time DESC").
		Offset(offset).
		Limit(filter.PageSize).
		Find(&instances).Error; err != nil {
		return nil, 0, err
	}

	return instances, total, nil
}

// UpdateStatus 更新状态
func (r *workflowScheduleInstanceRepository) UpdateStatus(ctx context.Context, instanceID string, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// 如果是完成或失败状态，更新结束时间和持续时间
	if status == "completed" || status == "failed" || status == "cancelled" {
		var instance models.WorkflowScheduleInstance
		if err := r.db.WithContext(ctx).Where("instance_id = ?", instanceID).First(&instance).Error; err != nil {
			return err
		}

		now := time.Now()
		updates["end_time"] = now
		if instance.StartTime != nil {
			duration := int(now.Sub(*instance.StartTime).Seconds())
			updates["duration"] = duration
		}
	}

	return r.db.WithContext(ctx).
		Model(&models.WorkflowScheduleInstance{}).
		Where("instance_id = ?", instanceID).
		Updates(updates).Error
}

// UpdateWorkflowInstanceIDs 更新工作流实例ID列表
func (r *workflowScheduleInstanceRepository) UpdateWorkflowInstanceIDs(ctx context.Context, instanceID string, workflowInstanceIDs interface{}) error {
	jsonData, err := json.Marshal(workflowInstanceIDs)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).
		Model(&models.WorkflowScheduleInstance{}).
		Where("instance_id = ?", instanceID).
		Update("workflow_instance_ids", jsonData).Error
}

// UpdateCounts 更新成功/失败计数
func (r *workflowScheduleInstanceRepository) UpdateCounts(ctx context.Context, instanceID string, successCount, failedCount int) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowScheduleInstance{}).
		Where("instance_id = ?", instanceID).
		Updates(map[string]interface{}{
			"success_count": successCount,
			"failed_count":  failedCount,
		}).Error
}

// GetStatistics 获取统计数据
func (r *workflowScheduleInstanceRepository) GetStatistics(ctx context.Context, scheduleID uint, startDate, endDate *time.Time) (*ScheduleInstanceStatistics, error) {
	stats := &ScheduleInstanceStatistics{
		ScheduleID:              scheduleID,
		TriggerTypeDistribution: make(map[string]int),
		DailyStatistics:         []DailyStatistic{},
	}

	query := r.db.WithContext(ctx).Model(&models.WorkflowScheduleInstance{}).
		Where("schedule_id = ?", scheduleID)

	if startDate != nil {
		query = query.Where("trigger_time >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("trigger_time <= ?", *endDate)
	}

	// 总实例数
	var totalInstances int64
	if err := query.Count(&totalInstances).Error; err != nil {
		return nil, err
	}
	stats.TotalInstances = int(totalInstances)

	// 各状态实例数
	var statusCounts []struct {
		Status string
		Count  int64
	}
	if err := query.Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, err
	}

	for _, sc := range statusCounts {
		switch sc.Status {
		case "completed":
			stats.CompletedInstances = int(sc.Count)
		case "failed":
			stats.FailedInstances = int(sc.Count)
		case "cancelled":
			stats.CancelledInstances = int(sc.Count)
		}
	}

	// 成功率
	if stats.TotalInstances > 0 {
		stats.SuccessRate = float64(stats.CompletedInstances) / float64(stats.TotalInstances) * 100
	}

	// 平均持续时间
	var avgDuration float64
	if err := query.Where("duration IS NOT NULL").
		Select("AVG(duration) as avg_duration").
		Scan(&avgDuration).Error; err != nil {
		return nil, err
	}
	stats.AvgDuration = int(avgDuration)

	// 工作流实例统计
	var instances []*models.WorkflowScheduleInstance
	if err := query.Find(&instances).Error; err != nil {
		return nil, err
	}

	for _, instance := range instances {
		stats.TotalWorkflowInstances += instance.SuccessCount + instance.FailedCount
		stats.WorkflowSuccessCount += instance.SuccessCount
		stats.WorkflowFailedCount += instance.FailedCount

		// 触发类型分布
		stats.TriggerTypeDistribution[string(instance.TriggerType)]++
	}

	// 工作流成功率
	if stats.TotalWorkflowInstances > 0 {
		stats.WorkflowSuccessRate = float64(stats.WorkflowSuccessCount) / float64(stats.TotalWorkflowInstances) * 100
	}

	// 每日统计
	var dailyStats []struct {
		Date      string
		Total     int64
		Completed int64
		Failed    int64
	}
	if err := query.Select("DATE(trigger_time) as date, COUNT(*) as total, " +
		"SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed, " +
		"SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed").
		Group("DATE(trigger_time)").
		Order("date DESC").
		Scan(&dailyStats).Error; err != nil {
		return nil, err
	}

	for _, ds := range dailyStats {
		successRate := 0.0
		if ds.Total > 0 {
			successRate = float64(ds.Completed) / float64(ds.Total) * 100
		}
		stats.DailyStatistics = append(stats.DailyStatistics, DailyStatistic{
			Date:        ds.Date,
			Total:       int(ds.Total),
			Completed:   int(ds.Completed),
			Failed:      int(ds.Failed),
			SuccessRate: successRate,
		})
	}

	return stats, nil
}

// BatchDelete 批量删除
func (r *workflowScheduleInstanceRepository) BatchDelete(ctx context.Context, instanceIDs []string) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("instance_id IN ? AND status IN ?", instanceIDs, []string{"completed", "failed", "cancelled"}).
		Delete(&models.WorkflowScheduleInstance{})

	return result.RowsAffected, result.Error
}
