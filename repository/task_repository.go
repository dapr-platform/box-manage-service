/*
 * @module repository/task_repository
 * @description 任务Repository实现，提供任务相关的数据访问操作
 * @architecture 数据访问层
 * @documentReference REQ-004: 任务调度系统
 * @stateFlow Service层 -> TaskRepository -> GORM -> 数据库
 * @rules 实现任务的CRUD操作、状态管理、统计查询等功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs DESIGN-001.md, context7 GORM最佳实践
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// taskRepository 任务Repository实现
type taskRepository struct {
	BaseRepository[models.Task]
	db *gorm.DB
}

// NewTaskRepository 创建Task Repository实例
func NewTaskRepository(db *gorm.DB) TaskRepository {
	return &taskRepository{
		BaseRepository: newBaseRepository[models.Task](db),
		db:             db,
	}
}

// FindByTaskID 根据任务ID查找任务
func (r *taskRepository) FindByTaskID(ctx context.Context, taskID string) (*models.Task, error) {
	var task models.Task
	err := r.db.WithContext(ctx).Where("task_id = ?", taskID).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// FindByExternalID 根据盒子ID和外部任务ID查找任务
func (r *taskRepository) FindByExternalID(ctx context.Context, boxID uint, externalID string) (*models.Task, error) {
	var task models.Task
	err := r.db.WithContext(ctx).Where("box_id = ? AND external_id = ?", boxID, externalID).First(&task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// FindByBoxID 根据盒子ID查找任务
func (r *taskRepository) FindByBoxID(ctx context.Context, boxID uint) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.WithContext(ctx).Where("box_id = ?", boxID).Find(&tasks).Error
	return tasks, err
}

// FindByStatus 根据状态查找任务
func (r *taskRepository) FindByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.WithContext(ctx).Where("status = ?", status).Find(&tasks).Error
	return tasks, err
}

// FindRunningTasks 查找运行中的任务
func (r *taskRepository) FindRunningTasks(ctx context.Context) ([]*models.Task, error) {
	return r.FindByStatus(ctx, models.TaskStatusRunning)
}

// UpdateStatus 更新任务状态
func (r *taskRepository) UpdateStatus(ctx context.Context, id uint, status models.TaskStatus) error {
	return r.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", id).Update("status", status).Error
}

// UpdateProgress 更新任务进度
func (r *taskRepository) UpdateProgress(ctx context.Context, id uint, totalFrames, inferenceCount, forwardSuccess, forwardFailed int64) error {
	updates := map[string]interface{}{
		"total_frames":    totalFrames,
		"inference_count": inferenceCount,
		"forward_success": forwardSuccess,
		"forward_failed":  forwardFailed,
	}
	return r.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", id).Updates(updates).Error
}

// GetTaskStatistics 获取任务统计
func (r *taskRepository) GetTaskStatistics(ctx context.Context) (map[string]interface{}, error) {
	var result struct {
		Total     int64 `json:"total"`
		Pending   int64 `json:"pending"`
		Running   int64 `json:"running"`
		Completed int64 `json:"completed"`
		Failed    int64 `json:"failed"`
		Paused    int64 `json:"paused"`
	}

	// 总任务数
	if err := r.db.WithContext(ctx).Model(&models.Task{}).Count(&result.Total).Error; err != nil {
		return nil, err
	}

	// 各状态任务数
	statuses := []models.TaskStatus{
		models.TaskStatusPending,
		models.TaskStatusRunning,
		models.TaskStatusCompleted,
		models.TaskStatusFailed,
		models.TaskStatusPaused,
	}

	for _, status := range statuses {
		var count int64
		if err := r.db.WithContext(ctx).Model(&models.Task{}).Where("status = ?", status).Count(&count).Error; err != nil {
			return nil, err
		}
		switch status {
		case models.TaskStatusPending:
			result.Pending = count
		case models.TaskStatusRunning:
			result.Running = count
		case models.TaskStatusCompleted:
			result.Completed = count
		case models.TaskStatusFailed:
			result.Failed = count
		case models.TaskStatusPaused:
			result.Paused = count
		}
	}

	return map[string]interface{}{
		"total":     result.Total,
		"pending":   result.Pending,
		"running":   result.Running,
		"completed": result.Completed,
		"failed":    result.Failed,
		"paused":    result.Paused,
	}, nil
}

// GetTaskStatusDistribution 获取任务状态分布
func (r *taskRepository) GetTaskStatusDistribution(ctx context.Context) (map[models.TaskStatus]int64, error) {
	result := make(map[models.TaskStatus]int64)

	rows, err := r.db.WithContext(ctx).
		Model(&models.Task{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status models.TaskStatus
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}

	return result, nil
}

// GetTasksByDateRange 根据日期范围获取任务
func (r *taskRepository) GetTasksByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.WithContext(ctx).
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Find(&tasks).Error
	return tasks, err
}

// FindByTags 根据标签查找任务
func (r *taskRepository) FindByTags(ctx context.Context, tags []string) ([]*models.Task, error) {
	var tasks []*models.Task

	// 构建JSON查询条件
	query := r.db.WithContext(ctx)
	for _, tag := range tags {
		query = query.Where("JSON_CONTAINS(tags, JSON_QUOTE(?))", tag)
	}

	err := query.Find(&tasks).Error
	return tasks, err
}

// GetAllTags 获取所有标签
func (r *taskRepository) GetAllTags(ctx context.Context) ([]string, error) {
	var tasks []*models.Task
	if err := r.db.WithContext(ctx).Select("tags").Find(&tasks).Error; err != nil {
		return nil, err
	}

	// 收集所有标签
	tagSet := make(map[string]bool)
	for _, task := range tasks {
		for _, tag := range task.GetTags() {
			tagSet[tag] = true
		}
	}

	// 转换为切片
	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	return tags, nil
}

// FindPendingTasks 查找待处理的任务
func (r *taskRepository) FindPendingTasks(ctx context.Context, limit int) ([]*models.Task, error) {
	var tasks []*models.Task
	query := r.db.WithContext(ctx).Where("status = ?", models.TaskStatusPending)
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Order("priority DESC, created_at ASC").Find(&tasks).Error
	return tasks, err
}

// FindTasksByModel 根据模型键查找任务
func (r *taskRepository) FindTasksByModel(ctx context.Context, modelKey string) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.WithContext(ctx).Where("model_key = ?", modelKey).Find(&tasks).Error
	return tasks, err
}

// FindTasksByPriority 根据优先级查找任务
func (r *taskRepository) FindTasksByPriority(ctx context.Context, priority int) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.WithContext(ctx).Where("priority = ?", priority).Find(&tasks).Error
	return tasks, err
}

// FindAutoScheduleTasks 查找启用自动调度的任务
func (r *taskRepository) FindAutoScheduleTasks(ctx context.Context, limit int) ([]*models.Task, error) {
	var tasks []*models.Task
	query := r.db.WithContext(ctx).
		Where("auto_schedule = ? AND status = ?", true, models.TaskStatusPending)
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Order("priority DESC, created_at ASC").Find(&tasks).Error
	return tasks, err
}

// FindTasksWithAffinityTags 根据亲和性标签查找任务
func (r *taskRepository) FindTasksWithAffinityTags(ctx context.Context, affinityTags []string) ([]*models.Task, error) {
	var tasks []*models.Task
	query := r.db.WithContext(ctx)

	for _, tag := range affinityTags {
		query = query.Where("JSON_CONTAINS(affinity_tags, JSON_QUOTE(?))", tag)
	}

	err := query.Find(&tasks).Error
	return tasks, err
}

// FindTasksCompatibleWithBox 查找与指定盒子兼容的任务
func (r *taskRepository) FindTasksCompatibleWithBox(ctx context.Context, boxID uint) ([]*models.Task, error) {
	// 首先获取盒子信息
	var box models.Box
	if err := r.db.WithContext(ctx).First(&box, boxID).Error; err != nil {
		return nil, err
	}

	// 获取盒子的标签
	boxTags := box.GetTags()
	if len(boxTags) == 0 {
		// 如果盒子没有标签，返回没有亲和性要求的任务
		var tasks []*models.Task
		err := r.db.WithContext(ctx).
			Where("status = ? AND (affinity_tags IS NULL OR JSON_LENGTH(affinity_tags) = 0)",
				models.TaskStatusPending).
			Find(&tasks).Error
		return tasks, err
	}

	// 查找匹配任何盒子标签的任务
	var tasks []*models.Task
	query := r.db.WithContext(ctx).Where("status = ?", models.TaskStatusPending)

	// 构建匹配任意标签的OR条件
	var conditions []string
	var args []interface{}
	for _, tag := range boxTags {
		conditions = append(conditions, "JSON_CONTAINS(affinity_tags, JSON_QUOTE(?))")
		args = append(args, tag)
	}

	// 同时包含没有亲和性要求的任务
	conditions = append(conditions, "(affinity_tags IS NULL OR JSON_LENGTH(affinity_tags) = 0)")

	query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
	err := query.Order("priority DESC, created_at ASC").Find(&tasks).Error
	return tasks, err
}

// SearchTasks 搜索任务
func (r *taskRepository) SearchTasks(ctx context.Context, keyword string, options *QueryOptions) ([]*models.Task, error) {
	var tasks []*models.Task
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
			"task_id LIKE ? OR name LIKE ? OR description LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%",
		)
	}

	err := query.Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// FindTasksWithBox 查找任务并包含盒子信息
func (r *taskRepository) FindTasksWithBox(ctx context.Context, options *QueryOptions) ([]*models.Task, error) {
	var tasks []*models.Task
	query := r.db.WithContext(ctx).Preload("Box")

	// 应用其他预加载选项
	if options != nil && len(options.Preload) > 0 {
		for _, preload := range options.Preload {
			if preload != "Box" { // 避免重复预加载
				query = query.Preload(preload)
			}
		}
	}

	err := query.Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// IsTaskIDExists 检查任务ID是否存在
func (r *taskRepository) IsTaskIDExists(ctx context.Context, taskID string, excludeIDs ...uint) (bool, error) {
	query := r.db.WithContext(ctx).Model(&models.Task{}).Where("task_id = ?", taskID)

	// 排除指定的ID（用于更新时检查）
	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

// BulkUpdateTaskStatus 批量更新任务状态
func (r *taskRepository) BulkUpdateTaskStatus(ctx context.Context, taskIDs []uint, status models.TaskStatus) error {
	return r.db.WithContext(ctx).Model(&models.Task{}).Where("id IN ?", taskIDs).Update("status", status).Error
}

// CountTasksByStatus 统计指定状态的任务数量
func (r *taskRepository) CountTasksByStatus(status models.TaskStatus) (int64, error) {
	var count int64
	err := r.db.Model(&models.Task{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// UpdateTaskProgress 更新任务进度
func (r *taskRepository) UpdateTaskProgress(ctx context.Context, taskID uint, progress float64) error {
	return r.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).Update("progress", progress).Error
}

// UpdateHeartbeat 更新任务心跳
func (r *taskRepository) UpdateHeartbeat(ctx context.Context, taskID uint, heartbeat time.Time) error {
	return r.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", taskID).Update("last_heartbeat", heartbeat).Error
}

// SetTaskError 设置任务错误信息
func (r *taskRepository) SetTaskError(ctx context.Context, id uint, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     models.TaskStatusFailed,
		"last_error": errorMsg,
	}
	return r.db.WithContext(ctx).Model(&models.Task{}).Where("id = ?", id).Updates(updates).Error
}

// CreateTaskExecution 创建任务执行记录
func (r *taskRepository) CreateTaskExecution(ctx context.Context, execution *models.TaskExecution) error {
	return r.db.WithContext(ctx).Create(execution).Error
}

// GetActiveTasksByBoxID 获取盒子的活跃任务
func (r *taskRepository) GetActiveTasksByBoxID(ctx context.Context, boxID uint) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.WithContext(ctx).
		Where("box_id = ? AND status IN ?", boxID, []models.TaskStatus{
			models.TaskStatusRunning,
			models.TaskStatusPending,
			models.TaskStatusScheduled,
		}).
		Find(&tasks).Error
	return tasks, err
}

// GetTaskExecutionHistory 获取任务执行历史
func (r *taskRepository) GetTaskExecutionHistory(ctx context.Context, taskID uint) ([]*models.TaskExecution, error) {
	var executions []*models.TaskExecution
	err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("started_at DESC").
		Find(&executions).Error
	return executions, err
}

// GetTaskPerformanceReport 获取任务性能报告
func (r *taskRepository) GetTaskPerformanceReport(ctx context.Context, startDate, endDate string) (map[string]interface{}, error) {
	// 获取时间范围内的任务
	var tasks []*models.Task
	query := r.db.WithContext(ctx).Model(&models.Task{})

	if startDate != "" && endDate != "" {
		query = query.Where("created_at BETWEEN ? AND ?", startDate, endDate)
	}

	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}

	// 计算统计数据
	var totalFrames, totalInferences, totalSuccess, totalFailed int64
	statusCounts := make(map[models.TaskStatus]int64)

	for _, task := range tasks {
		totalFrames += task.TotalFrames
		totalInferences += task.InferenceCount
		totalSuccess += task.ForwardSuccess
		totalFailed += task.ForwardFailed
		statusCounts[task.Status]++
	}

	report := map[string]interface{}{
		"total_tasks":         len(tasks),
		"total_frames":        totalFrames,
		"total_inferences":    totalInferences,
		"success_count":       totalSuccess,
		"failed_count":        totalFailed,
		"success_rate":        0.0,
		"status_distribution": statusCounts,
		"period": map[string]string{
			"start": startDate,
			"end":   endDate,
		},
	}

	// 计算成功率
	if totalInferences > 0 {
		report["success_rate"] = float64(totalSuccess) / float64(totalInferences) * 100
	}

	return report, nil
}

// UpdateTaskExecution 更新任务执行记录
func (r *taskRepository) UpdateTaskExecution(ctx context.Context, execution *models.TaskExecution) error {
	return r.db.WithContext(ctx).Save(execution).Error
}
