/*
 * @module repository/conversion_task_repository
 * @description 转换任务数据访问层，提供转换任务的CRUD操作
 * @architecture 数据访问层
 * @documentReference REQ-003: 模型转换功能
 * @stateFlow Service -> Repository -> Database
 * @rules 实现转换任务的数据库操作，包括查询、创建、更新、删除等
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs REQ-003.md, DESIGN-005.md
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// ConversionTaskRepository 转换任务数据访问接口
type ConversionTaskRepository interface {
	// 基本CRUD操作
	Create(ctx context.Context, task *models.ConversionTask) error
	GetByID(ctx context.Context, id uint) (*models.ConversionTask, error)
	GetByTaskID(ctx context.Context, taskID string) (*models.ConversionTask, error)
	Update(ctx context.Context, task *models.ConversionTask) error
	Delete(ctx context.Context, id uint) error

	// 状态和进度更新
	UpdateStatus(ctx context.Context, taskID string, status models.ConversionTaskStatus) error
	UpdateProgress(ctx context.Context, taskID string, progress int) error
	UpdateResourceUsage(ctx context.Context, taskID string, cpu float64, memory, disk int64) error

	// 查询操作
	GetByStatus(ctx context.Context, status models.ConversionTaskStatus) ([]*models.ConversionTask, error)
	GetPendingTasks(ctx context.Context) ([]*models.ConversionTask, error)
	GetRunningTasks(ctx context.Context) ([]*models.ConversionTask, error)
	GetByOriginalModelID(ctx context.Context, modelID uint) ([]*models.ConversionTask, error)
	GetByUserID(ctx context.Context, userID uint) ([]*models.ConversionTask, error)

	// 分页查询
	GetTaskList(ctx context.Context, req *GetConversionTaskListRequest) (*GetConversionTaskListResponse, error)
	GetTaskHistory(ctx context.Context, req *GetTaskHistoryRequest) (*GetTaskHistoryResponse, error)

	// 统计操作
	GetTaskStatistics(ctx context.Context, userID *uint) (*ConversionTaskStatistics, error)
	GetTaskCountByStatus(ctx context.Context, status models.ConversionTaskStatus) (int64, error)

	// 队列管理
	GetNextPendingTask(ctx context.Context) (*models.ConversionTask, error)
	GetQueueStatus(ctx context.Context) (*QueueStatus, error)

	// 清理操作
	CleanupExpiredTasks(ctx context.Context, expiredBefore time.Time) (int64, error)
	CleanupFailedTasks(ctx context.Context, olderThan time.Time) (int64, error)
}

// conversionTaskRepository 转换任务数据访问实现
type conversionTaskRepository struct {
	db *gorm.DB
}

// NewConversionTaskRepository 创建转换任务数据访问实例
func NewConversionTaskRepository(db *gorm.DB) ConversionTaskRepository {
	return &conversionTaskRepository{db: db}
}

// Create 创建转换任务
func (r *conversionTaskRepository) Create(ctx context.Context, task *models.ConversionTask) error {
	// 生成任务ID
	task.TaskID = models.GenerateTaskID()

	return r.db.WithContext(ctx).Create(task).Error
}

// GetByID 根据ID获取转换任务
func (r *conversionTaskRepository) GetByID(ctx context.Context, id uint) (*models.ConversionTask, error) {
	var task models.ConversionTask
	err := r.db.WithContext(ctx).
		Preload("OriginalModel").
		First(&task, id).Error

	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetByTaskID 根据任务ID获取转换任务
func (r *conversionTaskRepository) GetByTaskID(ctx context.Context, taskID string) (*models.ConversionTask, error) {
	var task models.ConversionTask
	err := r.db.WithContext(ctx).
		Preload("OriginalModel").
		Where("task_id = ?", taskID).
		First(&task).Error

	if err != nil {
		return nil, err
	}
	return &task, nil
}

// Update 更新转换任务
func (r *conversionTaskRepository) Update(ctx context.Context, task *models.ConversionTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

// Delete 删除转换任务
func (r *conversionTaskRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ConversionTask{}, id).Error
}

// UpdateStatus 更新任务状态
func (r *conversionTaskRepository) UpdateStatus(ctx context.Context, taskID string, status models.ConversionTaskStatus) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	// 根据状态设置相应的时间戳
	switch status {
	case models.ConversionTaskStatusRunning:
		updates["start_time"] = time.Now()
	case models.ConversionTaskStatusCompleted, models.ConversionTaskStatusFailed:
		updates["end_time"] = time.Now()
	}

	return r.db.WithContext(ctx).
		Model(&models.ConversionTask{}).
		Where("task_id = ?", taskID).
		Updates(updates).Error
}

// UpdateProgress 更新任务进度
func (r *conversionTaskRepository) UpdateProgress(ctx context.Context, taskID string, progress int) error {
	return r.db.WithContext(ctx).
		Model(&models.ConversionTask{}).
		Where("task_id = ?", taskID).
		Updates(map[string]interface{}{
			"progress":   progress,
			"updated_at": time.Now(),
		}).Error
}

// UpdateResourceUsage 更新资源使用情况
func (r *conversionTaskRepository) UpdateResourceUsage(ctx context.Context, taskID string, cpu float64, memory, disk int64) error {
	return r.db.WithContext(ctx).
		Model(&models.ConversionTask{}).
		Where("task_id = ?", taskID).
		Updates(map[string]interface{}{
			"cpu_usage":    cpu,
			"memory_usage": memory,
			"disk_usage":   disk,
			"updated_at":   time.Now(),
		}).Error
}

// GetByStatus 根据状态获取任务列表
func (r *conversionTaskRepository) GetByStatus(ctx context.Context, status models.ConversionTaskStatus) ([]*models.ConversionTask, error) {
	var tasks []*models.ConversionTask
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at ASC").
		Find(&tasks).Error

	return tasks, err
}

// GetPendingTasks 获取待处理任务
func (r *conversionTaskRepository) GetPendingTasks(ctx context.Context) ([]*models.ConversionTask, error) {
	var tasks []*models.ConversionTask
	err := r.db.WithContext(ctx).
		Where("status IN ?", []models.ConversionTaskStatus{
			models.ConversionTaskStatusPending,
			models.ConversionTaskStatusPending,
		}).
		Order("created_at ASC").
		Find(&tasks).Error

	return tasks, err
}

// GetRunningTasks 获取运行中任务
func (r *conversionTaskRepository) GetRunningTasks(ctx context.Context) ([]*models.ConversionTask, error) {
	return r.GetByStatus(ctx, models.ConversionTaskStatusRunning)
}

// GetByOriginalModelID 根据原始模型ID获取转换任务
func (r *conversionTaskRepository) GetByOriginalModelID(ctx context.Context, modelID uint) ([]*models.ConversionTask, error) {
	var tasks []*models.ConversionTask
	err := r.db.WithContext(ctx).
		Where("original_model_id = ?", modelID).
		Order("created_at DESC").
		Find(&tasks).Error

	return tasks, err
}

// GetByUserID 根据用户ID获取转换任务
func (r *conversionTaskRepository) GetByUserID(ctx context.Context, userID uint) ([]*models.ConversionTask, error) {
	var tasks []*models.ConversionTask
	err := r.db.WithContext(ctx).
		Where("created_by = ?", userID).
		Order("created_at DESC").
		Find(&tasks).Error

	return tasks, err
}

// GetTaskList 分页获取任务列表
func (r *conversionTaskRepository) GetTaskList(ctx context.Context, req *GetConversionTaskListRequest) (*GetConversionTaskListResponse, error) {
	var tasks []*models.ConversionTask
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ConversionTask{})

	// 构建查询条件
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.UserID != nil {
		query = query.Where("created_by = ?", *req.UserID)
	}
	if req.OriginalModelID != nil {
		query = query.Where("original_model_id = ?", *req.OriginalModelID)
	}

	if !req.StartTime.IsZero() {
		query = query.Where("created_at >= ?", req.StartTime)
	}
	if !req.EndTime.IsZero() {
		query = query.Where("created_at <= ?", req.EndTime)
	}
	if req.Keyword != "" {
		query = query.Joins("LEFT JOIN original_models ON conversion_tasks.original_model_id = original_models.id").
			Where("original_models.name LIKE ? OR conversion_tasks.task_id LIKE ?",
				"%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	err := query.
		Preload("OriginalModel").
		Order("created_at DESC").
		Offset(offset).
		Limit(req.PageSize).
		Find(&tasks).Error

	if err != nil {
		return nil, err
	}

	return &GetConversionTaskListResponse{
		Tasks:    tasks,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetTaskHistory 获取任务历史记录
func (r *conversionTaskRepository) GetTaskHistory(ctx context.Context, req *GetTaskHistoryRequest) (*GetTaskHistoryResponse, error) {
	var tasks []*models.ConversionTask
	var total int64

	query := r.db.WithContext(ctx).Model(&models.ConversionTask{}).
		Where("status IN ?", []models.ConversionTaskStatus{
			models.ConversionTaskStatusCompleted,
			models.ConversionTaskStatusFailed,
			models.ConversionTaskStatusFailed,
		})

	// 构建查询条件
	if req.UserID != nil {
		query = query.Where("created_by = ?", *req.UserID)
	}
	if !req.StartTime.IsZero() {
		query = query.Where("created_at >= ?", req.StartTime)
	}
	if !req.EndTime.IsZero() {
		query = query.Where("created_at <= ?", req.EndTime)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	err := query.
		Preload("OriginalModel").
		Order("created_at DESC").
		Offset(offset).
		Limit(req.PageSize).
		Find(&tasks).Error

	if err != nil {
		return nil, err
	}

	return &GetTaskHistoryResponse{
		Tasks:    tasks,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetTaskStatistics 获取任务统计信息
func (r *conversionTaskRepository) GetTaskStatistics(ctx context.Context, userID *uint) (*ConversionTaskStatistics, error) {
	var stats ConversionTaskStatistics

	query := r.db.WithContext(ctx).Model(&models.ConversionTask{})
	if userID != nil {
		query = query.Where("created_by = ?", *userID)
	}

	// 总任务数
	if err := query.Count(&stats.TotalTasks).Error; err != nil {
		return nil, err
	}

	// 各状态任务数
	statusCounts := make(map[string]int64)
	rows, err := query.Select("status, COUNT(*) as count").Group("status").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		statusCounts[status] = count
	}

	stats.PendingTasks = statusCounts["pending"]
	stats.RunningTasks = statusCounts["running"]
	stats.CompletedTasks = statusCounts["completed"]
	stats.FailedTasks = statusCounts["failed"]
	stats.CancelledTasks = statusCounts["cancelled"]

	// 成功率
	if stats.TotalTasks > 0 {
		stats.SuccessRate = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}

	// 平均耗时（只计算已完成的任务）
	var avgDuration float64
	err = query.Where("status = ? AND start_time IS NOT NULL AND end_time IS NOT NULL",
		models.ConversionTaskStatusCompleted).
		Select("AVG(EXTRACT(EPOCH FROM (end_time - start_time)))").
		Scan(&avgDuration).Error

	if err != nil {
		return nil, err
	}
	stats.AverageExecutionTime = time.Duration(avgDuration) * time.Second

	return &stats, nil
}

// GetTaskCountByStatus 根据状态获取任务数量
func (r *conversionTaskRepository) GetTaskCountByStatus(ctx context.Context, status models.ConversionTaskStatus) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ConversionTask{}).
		Where("status = ?", status).
		Count(&count).Error

	return count, err
}

// GetNextPendingTask 获取下一个待处理任务（按优先级和创建时间排序）
func (r *conversionTaskRepository) GetNextPendingTask(ctx context.Context) (*models.ConversionTask, error) {
	var task models.ConversionTask
	err := r.db.WithContext(ctx).
		Where("status = ?", models.ConversionTaskStatusPending).
		Order("created_at ASC").
		First(&task).Error

	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetQueueStatus 获取队列状态
func (r *conversionTaskRepository) GetQueueStatus(ctx context.Context) (*QueueStatus, error) {
	var status QueueStatus

	// 获取各状态任务数量
	pendingCount, err := r.GetTaskCountByStatus(ctx, models.ConversionTaskStatusPending)
	if err != nil {
		return nil, err
	}
	status.PendingCount = int(pendingCount)

	queuedCount, err := r.GetTaskCountByStatus(ctx, models.ConversionTaskStatusPending)
	if err != nil {
		return nil, err
	}
	status.QueuedCount = int(queuedCount)

	runningCount, err := r.GetTaskCountByStatus(ctx, models.ConversionTaskStatusRunning)
	if err != nil {
		return nil, err
	}
	status.RunningCount = int(runningCount)

	// 总队列长度
	status.TotalQueueLength = status.PendingCount + status.QueuedCount

	return &status, nil
}

// CleanupExpiredTasks 清理过期任务
func (r *conversionTaskRepository) CleanupExpiredTasks(ctx context.Context, expiredBefore time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("status IN ? AND created_at < ?",
			[]models.ConversionTaskStatus{
				models.ConversionTaskStatusCompleted,
				models.ConversionTaskStatusFailed,
				models.ConversionTaskStatusFailed,
			},
			expiredBefore).
		Delete(&models.ConversionTask{})

	return result.RowsAffected, result.Error
}

// CleanupFailedTasks 清理失败任务
func (r *conversionTaskRepository) CleanupFailedTasks(ctx context.Context, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", models.ConversionTaskStatusFailed, olderThan).
		Delete(&models.ConversionTask{})

	return result.RowsAffected, result.Error
}

// 请求和响应结构体

// GetConversionTaskListRequest 获取转换任务列表请求
type GetConversionTaskListRequest struct {
	Page            int    `json:"page" example:"1"`
	PageSize        int    `json:"page_size" example:"20"`
	Status          string `json:"status" example:"running"`
	UserID          *uint  `json:"user_id" example:"1"`
	OriginalModelID *uint  `json:"original_model_id" example:"1"`

	StartTime time.Time `json:"start_time" example:"2025-01-01T00:00:00Z"`
	EndTime   time.Time `json:"end_time" example:"2025-01-31T23:59:59Z"`
	Keyword   string    `json:"keyword" example:"yolo"`
}

// GetConversionTaskListResponse 获取转换任务列表响应
type GetConversionTaskListResponse struct {
	Tasks    []*models.ConversionTask `json:"tasks"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

// GetTaskHistoryRequest 获取任务历史请求
type GetTaskHistoryRequest struct {
	Page      int       `json:"page" example:"1"`
	PageSize  int       `json:"page_size" example:"20"`
	UserID    *uint     `json:"user_id" example:"1"`
	StartTime time.Time `json:"start_time" example:"2025-01-01T00:00:00Z"`
	EndTime   time.Time `json:"end_time" example:"2025-01-31T23:59:59Z"`
}

// GetTaskHistoryResponse 获取任务历史响应
type GetTaskHistoryResponse struct {
	Tasks    []*models.ConversionTask `json:"tasks"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

// ConversionTaskStatistics 转换任务统计信息
type ConversionTaskStatistics struct {
	TotalTasks           int64         `json:"total_tasks"`
	PendingTasks         int64         `json:"pending_tasks"`
	RunningTasks         int64         `json:"running_tasks"`
	CompletedTasks       int64         `json:"completed_tasks"`
	FailedTasks          int64         `json:"failed_tasks"`
	CancelledTasks       int64         `json:"cancelled_tasks"`
	SuccessRate          float64       `json:"success_rate"`
	AverageExecutionTime time.Duration `json:"average_execution_time"`
}

// QueueStatus 队列状态
type QueueStatus struct {
	PendingCount     int `json:"pending_count"`
	QueuedCount      int `json:"queued_count"`
	RunningCount     int `json:"running_count"`
	TotalQueueLength int `json:"total_queue_length"`
	ActiveWorkers    int `json:"active_workers"`
	TotalWorkers     int `json:"total_workers"`
}
