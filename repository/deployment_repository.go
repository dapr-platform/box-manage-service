/*
 * @module repository/deployment_repository
 * @description 部署任务Repository实现，提供部署任务相关的数据访问操作
 * @architecture 数据访问层
 * @documentReference REQ-004: 任务调度系统
 * @stateFlow Service层 -> DeploymentRepository -> GORM -> 数据库
 * @rules 实现部署任务的CRUD操作、状态管理、日志查询等功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs DESIGN-001.md
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// deploymentRepository 部署任务Repository实现
type deploymentRepository struct {
	BaseRepository[models.DeploymentTask]
	db *gorm.DB
}

// NewDeploymentRepository 创建部署任务Repository实例
func NewDeploymentRepository(db *gorm.DB) DeploymentRepository {
	return &deploymentRepository{
		BaseRepository: newBaseRepository[models.DeploymentTask](db),
		db:             db,
	}
}

// FindByStatus 根据状态查找部署任务
func (r *deploymentRepository) FindByStatus(ctx context.Context, status models.DeploymentTaskStatus) ([]*models.DeploymentTask, error) {
	var tasks []*models.DeploymentTask
	err := r.db.WithContext(ctx).Where("status = ?", status).Order("priority DESC, created_at DESC").Find(&tasks).Error
	return tasks, err
}

// FindByCreatedBy 根据创建者查找部署任务
func (r *deploymentRepository) FindByCreatedBy(ctx context.Context, userID uint) ([]*models.DeploymentTask, error) {
	var tasks []*models.DeploymentTask
	err := r.db.WithContext(ctx).Where("created_by = ?", userID).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// UpdateStatus 更新部署任务状态
func (r *deploymentRepository) UpdateStatus(ctx context.Context, id uint, status models.DeploymentTaskStatus) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	// 如果是完成状态，设置完成时间
	if status == models.DeploymentTaskStatusCompleted ||
		status == models.DeploymentTaskStatusFailed ||
		status == models.DeploymentTaskStatusCancelled {
		updates["completed_at"] = time.Now()
	}

	return r.db.WithContext(ctx).Model(&models.DeploymentTask{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateProgress 更新进度和消息
func (r *deploymentRepository) UpdateProgress(ctx context.Context, id uint, progress float64, message string) error {
	updates := map[string]interface{}{
		"progress":   progress,
		"message":    message,
		"updated_at": time.Now(),
	}
	return r.db.WithContext(ctx).Model(&models.DeploymentTask{}).Where("id = ?", id).Updates(updates).Error
}

// FindRunningTasks 查找运行中的部署任务
func (r *deploymentRepository) FindRunningTasks(ctx context.Context) ([]*models.DeploymentTask, error) {
	return r.FindByStatus(ctx, models.DeploymentTaskStatusRunning)
}

// FindPendingTasks 查找待处理的部署任务
func (r *deploymentRepository) FindPendingTasks(ctx context.Context, limit int) ([]*models.DeploymentTask, error) {
	var tasks []*models.DeploymentTask
	query := r.db.WithContext(ctx).Where("status = ?", models.DeploymentTaskStatusPending)
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Order("priority DESC, created_at ASC").Find(&tasks).Error
	return tasks, err
}

// AddExecutionLog 添加执行日志
func (r *deploymentRepository) AddExecutionLog(ctx context.Context, id uint, log models.DeploymentLog) error {
	// 获取当前任务
	var task models.DeploymentTask
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		return err
	}

	// 添加日志
	task.ExecutionLogs = append(task.ExecutionLogs, log)

	// 限制日志数量，保留最新的1000条
	if len(task.ExecutionLogs) > 1000 {
		task.ExecutionLogs = task.ExecutionLogs[len(task.ExecutionLogs)-1000:]
	}

	return r.db.WithContext(ctx).Model(&task).Update("execution_logs", task.ExecutionLogs).Error
}

// AddResult 添加部署结果
func (r *deploymentRepository) AddResult(ctx context.Context, id uint, result models.DeploymentResult) error {
	// 获取当前任务
	var task models.DeploymentTask
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		return err
	}

	// 添加结果
	task.Results = append(task.Results, result)

	// 更新统计信息
	if result.Success {
		task.CompletedTasks++
	} else {
		task.FailedTasks++
	}

	// 更新进度
	if task.TotalTasks > 0 {
		completed := task.CompletedTasks + task.FailedTasks + task.SkippedTasks
		task.Progress = float64(completed) / float64(task.TotalTasks) * 100
	}

	updates := map[string]interface{}{
		"results":         task.Results,
		"completed_tasks": task.CompletedTasks,
		"failed_tasks":    task.FailedTasks,
		"progress":        task.Progress,
		"updated_at":      time.Now(),
	}

	return r.db.WithContext(ctx).Model(&task).Where("id = ?", id).Updates(updates).Error
}

// GetLogs 获取部署任务日志
func (r *deploymentRepository) GetLogs(ctx context.Context, id uint, limit int) ([]models.DeploymentLog, error) {
	var task models.DeploymentTask
	if err := r.db.WithContext(ctx).Select("execution_logs").First(&task, id).Error; err != nil {
		return nil, err
	}

	logs := task.ExecutionLogs
	if limit > 0 && len(logs) > limit {
		// 返回最新的日志
		logs = logs[len(logs)-limit:]
	}

	return logs, nil
}

// GetResults 获取部署结果
func (r *deploymentRepository) GetResults(ctx context.Context, id uint) ([]models.DeploymentResult, error) {
	var task models.DeploymentTask
	if err := r.db.WithContext(ctx).Select("results").First(&task, id).Error; err != nil {
		return nil, err
	}

	return task.Results, nil
}

// GetStatistics 获取部署任务统计
func (r *deploymentRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	var result struct {
		Total     int64 `json:"total"`
		Pending   int64 `json:"pending"`
		Running   int64 `json:"running"`
		Completed int64 `json:"completed"`
		Failed    int64 `json:"failed"`
		Cancelled int64 `json:"cancelled"`
	}

	// 总任务数
	if err := r.db.WithContext(ctx).Model(&models.DeploymentTask{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	// 各状态任务数
	statuses := []models.DeploymentTaskStatus{
		models.DeploymentTaskStatusPending,
		models.DeploymentTaskStatusRunning,
		models.DeploymentTaskStatusCompleted,
		models.DeploymentTaskStatusFailed,
		models.DeploymentTaskStatusCancelled,
	}

	for _, status := range statuses {
		var count int64
		if err := r.db.WithContext(ctx).Model(&models.DeploymentTask{}).Where("status = ?", status).Count(&count).Error; err != nil {
			return nil, err
		}
		switch status {
		case models.DeploymentTaskStatusPending:
			result.Pending = count
		case models.DeploymentTaskStatusRunning:
			result.Running = count
		case models.DeploymentTaskStatusCompleted:
			result.Completed = count
		case models.DeploymentTaskStatusFailed:
			result.Failed = count
		case models.DeploymentTaskStatusCancelled:
			result.Cancelled = count
		}
	}

	return map[string]interface{}{
		"total":     result.Total,
		"pending":   result.Pending,
		"running":   result.Running,
		"completed": result.Completed,
		"failed":    result.Failed,
		"cancelled": result.Cancelled,
	}, nil
}

// GetStatusDistribution 获取状态分布
func (r *deploymentRepository) GetStatusDistribution(ctx context.Context) (map[models.DeploymentTaskStatus]int64, error) {
	result := make(map[models.DeploymentTaskStatus]int64)

	rows, err := r.db.WithContext(ctx).
		Model(&models.DeploymentTask{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status models.DeploymentTaskStatus
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}

	return result, nil
}

// GetPerformanceReport 获取性能报告
func (r *deploymentRepository) GetPerformanceReport(ctx context.Context, startDate, endDate string) (map[string]interface{}, error) {
	var tasks []*models.DeploymentTask
	query := r.db.WithContext(ctx).Model(&models.DeploymentTask{})

	if startDate != "" && endDate != "" {
		query = query.Where("created_at BETWEEN ? AND ?", startDate, endDate)
	}

	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}

	// 计算统计数据
	var totalTasks, totalCompleted, totalFailed int64
	var totalDuration time.Duration
	successRates := make([]float64, 0)

	for _, task := range tasks {
		totalTasks++
		if task.Status == models.DeploymentTaskStatusCompleted {
			totalCompleted++
		} else if task.Status == models.DeploymentTaskStatusFailed {
			totalFailed++
		}

		if task.StartedAt != nil {
			duration := task.GetDuration()
			totalDuration += duration
		}

		if task.TotalTasks > 0 {
			successRate := task.GetSuccessRate()
			successRates = append(successRates, successRate)
		}
	}

	// 计算平均成功率
	var avgSuccessRate float64
	if len(successRates) > 0 {
		var sum float64
		for _, rate := range successRates {
			sum += rate
		}
		avgSuccessRate = sum / float64(len(successRates))
	}

	// 计算平均执行时间
	var avgDuration float64
	if totalTasks > 0 {
		avgDuration = totalDuration.Seconds() / float64(totalTasks)
	}

	return map[string]interface{}{
		"total_deployments":     totalTasks,
		"completed_deployments": totalCompleted,
		"failed_deployments":    totalFailed,
		"success_rate":          avgSuccessRate,
		"avg_duration_seconds":  avgDuration,
		"period": map[string]string{
			"start": startDate,
			"end":   endDate,
		},
	}, nil
}

// SearchTasks 搜索部署任务
func (r *deploymentRepository) SearchTasks(ctx context.Context, keyword string, options *QueryOptions) ([]*models.DeploymentTask, error) {
	var tasks []*models.DeploymentTask
	query := r.db.WithContext(ctx)

	// 应用预加载选项
	if options != nil && len(options.Preload) > 0 {
		for _, preload := range options.Preload {
			query = query.Preload(preload)
		}
	}

	// 搜索条件
	if keyword != "" {
		query = query.Where(
			"name LIKE ? OR description LIKE ? OR message LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%",
		)
	}

	err := query.Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// FindByDateRange 根据日期范围查找部署任务
func (r *deploymentRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*models.DeploymentTask, error) {
	var tasks []*models.DeploymentTask
	err := r.db.WithContext(ctx).
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Order("created_at DESC").
		Find(&tasks).Error
	return tasks, err
}

// FindByPriority 根据优先级查找部署任务
func (r *deploymentRepository) FindByPriority(ctx context.Context, priority int) ([]*models.DeploymentTask, error) {
	var tasks []*models.DeploymentTask
	err := r.db.WithContext(ctx).Where("priority = ?", priority).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// CleanupCompletedTasks 清理已完成的任务
func (r *deploymentRepository) CleanupCompletedTasks(ctx context.Context, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("status IN ? AND completed_at < ?",
			[]models.DeploymentTaskStatus{
				models.DeploymentTaskStatusCompleted,
				models.DeploymentTaskStatusFailed,
				models.DeploymentTaskStatusCancelled,
			},
			olderThan).
		Delete(&models.DeploymentTask{})

	return result.RowsAffected, result.Error
}

// CleanupLogs 清理部署任务日志，保留最新的条目
func (r *deploymentRepository) CleanupLogs(ctx context.Context, deploymentID uint, keepLatest int) error {
	var task models.DeploymentTask
	if err := r.db.WithContext(ctx).First(&task, deploymentID).Error; err != nil {
		return err
	}

	if len(task.ExecutionLogs) <= keepLatest {
		return nil // 无需清理
	}

	// 保留最新的日志
	task.ExecutionLogs = task.ExecutionLogs[len(task.ExecutionLogs)-keepLatest:]

	return r.db.WithContext(ctx).Model(&task).Update("execution_logs", task.ExecutionLogs).Error
}
