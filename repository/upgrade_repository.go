/*
 * @module repository/upgrade_repository
 * @description 升级Repository实现，提供升级任务相关的数据访问操作
 * @architecture 数据访问层
 * @documentReference REQ-001: 盒子管理功能 - 软件升级
 * @stateFlow Service层 -> UpgradeRepository -> GORM -> 数据库
 * @rules 实现升级任务的CRUD操作、状态管理、批量升级管理等功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs DESIGN-001.md, context7 GORM最佳实践
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// upgradeRepository 升级Repository实现
type upgradeRepository struct {
	BaseRepository[models.UpgradeTask]
	db *gorm.DB
}

// NewUpgradeRepository 创建Upgrade Repository实例
func NewUpgradeRepository(db *gorm.DB) UpgradeRepository {
	return &upgradeRepository{
		BaseRepository: newBaseRepository[models.UpgradeTask](db),
		db:             db,
	}
}

// FindByBoxID 根据盒子ID查找升级任务
func (r *upgradeRepository) FindByBoxID(ctx context.Context, boxID uint) ([]*models.UpgradeTask, error) {
	var upgradeTasks []*models.UpgradeTask
	err := r.db.WithContext(ctx).
		Where("box_id = ?", boxID).
		Preload("Box").
		Preload("BatchUpgradeTask").
		Order("created_at DESC").
		Find(&upgradeTasks).Error
	return upgradeTasks, err
}

// FindByStatus 根据状态查找升级任务
func (r *upgradeRepository) FindByStatus(ctx context.Context, status models.UpgradeStatus) ([]*models.UpgradeTask, error) {
	var upgradeTasks []*models.UpgradeTask
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Preload("Box").
		Preload("BatchUpgradeTask").
		Order("created_at DESC").
		Find(&upgradeTasks).Error
	return upgradeTasks, err
}

// FindPendingUpgrades 查找待执行的升级任务
func (r *upgradeRepository) FindPendingUpgrades(ctx context.Context) ([]*models.UpgradeTask, error) {
	var upgradeTasks []*models.UpgradeTask
	err := r.db.WithContext(ctx).
		Where("status = ?", models.UpgradeStatusPending).
		Preload("Box").
		Preload("BatchUpgradeTask").
		Order("created_at ASC"). // 按创建时间升序，优先处理较早的任务
		Find(&upgradeTasks).Error
	return upgradeTasks, err
}

// FindByBatchID 根据批量升级ID查找升级任务
func (r *upgradeRepository) FindByBatchID(ctx context.Context, batchID uint) ([]*models.UpgradeTask, error) {
	var upgradeTasks []*models.UpgradeTask
	err := r.db.WithContext(ctx).
		Where("batch_upgrade_task_id = ?", batchID).
		Preload("Box").
		Order("created_at ASC").
		Find(&upgradeTasks).Error
	return upgradeTasks, err
}

// UpdateStatus 更新升级任务状态
func (r *upgradeRepository) UpdateStatus(ctx context.Context, id uint, status models.UpgradeStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// 根据状态设置相应的时间字段
	now := time.Now()
	switch status {
	case models.UpgradeStatusRunning:
		updates["started_at"] = now
	case models.UpgradeStatusCompleted, models.UpgradeStatusFailed,
		models.UpgradeStatusCancelled, models.UpgradeStatusRolledback:
		updates["completed_at"] = now
	}

	return r.db.WithContext(ctx).
		Model(&models.UpgradeTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateProgress 更新升级进度
func (r *upgradeRepository) UpdateProgress(ctx context.Context, id uint, progress int) error {
	return r.db.WithContext(ctx).
		Model(&models.UpgradeTask{}).
		Where("id = ?", id).
		Update("progress", progress).Error
}

// SetError 设置升级任务错误信息
func (r *upgradeRepository) SetError(ctx context.Context, id uint, errorMsg, errorDetail string) error {
	return r.db.WithContext(ctx).
		Model(&models.UpgradeTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        models.UpgradeStatusFailed,
			"error_message": errorMsg,
			"error_detail":  errorDetail,
			"completed_at":  time.Now(),
		}).Error
}

// CreateBatchUpgrade 创建批量升级任务
func (r *upgradeRepository) CreateBatchUpgrade(ctx context.Context, batch *models.BatchUpgradeTask) error {
	return r.db.WithContext(ctx).Create(batch).Error
}

// GetBatchUpgrade 获取批量升级任务
func (r *upgradeRepository) GetBatchUpgrade(ctx context.Context, id uint) (*models.BatchUpgradeTask, error) {
	var batchTask models.BatchUpgradeTask
	err := r.db.WithContext(ctx).
		Preload("UpgradeTasks").
		Preload("UpgradeTasks.Box").
		First(&batchTask, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &batchTask, nil
}

// UpdateBatchStatus 更新批量升级状态
func (r *upgradeRepository) UpdateBatchStatus(ctx context.Context, id uint, status models.UpgradeStatus) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// 根据状态设置时间
	now := time.Now()
	switch status {
	case models.UpgradeStatusRunning:
		updates["started_at"] = now
	case models.UpgradeStatusCompleted, models.UpgradeStatusFailed, models.UpgradeStatusCancelled:
		updates["completed_at"] = now
	}

	return r.db.WithContext(ctx).
		Model(&models.BatchUpgradeTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// GetUpgradeStatistics 获取升级统计信息
func (r *upgradeRepository) GetUpgradeStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总升级任务数
	var totalCount int64
	if err := r.db.WithContext(ctx).Model(&models.UpgradeTask{}).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total"] = totalCount

	// 按状态统计
	statusStats := make(map[string]int64)
	type statusCount struct {
		Status models.UpgradeStatus `json:"status"`
		Count  int64                `json:"count"`
	}

	var statusResults []statusCount
	err := r.db.WithContext(ctx).
		Model(&models.UpgradeTask{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&statusResults).Error

	if err != nil {
		return nil, err
	}

	for _, result := range statusResults {
		statusStats[string(result.Status)] = result.Count
	}
	stats["by_status"] = statusStats

	// 今日升级任务数
	today := time.Now().Truncate(24 * time.Hour)
	var todayCount int64
	if err := r.db.WithContext(ctx).Model(&models.UpgradeTask{}).
		Where("created_at >= ?", today).
		Count(&todayCount).Error; err != nil {
		return nil, err
	}
	stats["today_created"] = todayCount

	// 成功率
	var completedCount int64
	if err := r.db.WithContext(ctx).Model(&models.UpgradeTask{}).
		Where("status = ?", models.UpgradeStatusCompleted).
		Count(&completedCount).Error; err != nil {
		return nil, err
	}

	var finishedCount int64
	if err := r.db.WithContext(ctx).Model(&models.UpgradeTask{}).
		Where("status IN ?", []models.UpgradeStatus{
			models.UpgradeStatusCompleted, models.UpgradeStatusFailed, models.UpgradeStatusCancelled,
		}).Count(&finishedCount).Error; err != nil {
		return nil, err
	}

	successRate := float64(0)
	if finishedCount > 0 {
		successRate = float64(completedCount) / float64(finishedCount) * 100
	}
	stats["success_rate"] = successRate

	// 批量升级统计
	var batchCount int64
	if err := r.db.WithContext(ctx).Model(&models.BatchUpgradeTask{}).Count(&batchCount).Error; err != nil {
		return nil, err
	}
	stats["batch_total"] = batchCount

	return stats, nil
}

// GetUpgradeHistory 获取升级历史记录
func (r *upgradeRepository) GetUpgradeHistory(ctx context.Context, boxID uint, limit int) ([]*models.UpgradeTask, error) {
	var upgradeTasks []*models.UpgradeTask

	query := r.db.WithContext(ctx).
		Preload("Box").
		Preload("BatchUpgradeTask").
		Order("created_at DESC")

	if boxID > 0 {
		query = query.Where("box_id = ?", boxID)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&upgradeTasks).Error
	return upgradeTasks, err
}

// GetRunningUpgrades 获取正在运行的升级任务
func (r *upgradeRepository) GetRunningUpgrades(ctx context.Context) ([]*models.UpgradeTask, error) {
	var upgradeTasks []*models.UpgradeTask
	err := r.db.WithContext(ctx).
		Where("status = ?", models.UpgradeStatusRunning).
		Preload("Box").
		Preload("BatchUpgradeTask").
		Order("started_at ASC").
		Find(&upgradeTasks).Error
	return upgradeTasks, err
}

// CanRollback 检查是否可以回滚
func (r *upgradeRepository) CanRollback(ctx context.Context, id uint) (bool, error) {
	var task models.UpgradeTask
	err := r.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return false, err
	}

	return task.CanRollback && task.Status == models.UpgradeStatusCompleted, nil
}

// CreateRollbackTask 创建回滚任务
func (r *upgradeRepository) CreateRollbackTask(ctx context.Context, originalTaskID uint, createdBy uint) (*models.UpgradeTask, error) {
	// 获取原始任务信息
	var originalTask models.UpgradeTask
	err := r.db.WithContext(ctx).First(&originalTask, originalTaskID).Error
	if err != nil {
		return nil, err
	}

	// 创建回滚任务
	rollbackTask := &models.UpgradeTask{
		BoxID:          originalTask.BoxID,
		VersionFrom:    originalTask.VersionTo,
		VersionTo:      originalTask.RollbackVersion,
		Status:         models.UpgradeStatusPending,
		ProgramFile:    "", // 需要另外设置回滚包路径
		Force:          false,
		RollbackTaskID: &originalTaskID,
		CreatedBy:      createdBy,
	}

	err = r.db.WithContext(ctx).Create(rollbackTask).Error
	if err != nil {
		return nil, err
	}

	return rollbackTask, nil
}

// GetBatchUpgrades 获取批量升级任务列表
func (r *upgradeRepository) GetBatchUpgrades(ctx context.Context, options *QueryOptions) ([]*models.BatchUpgradeTask, error) {
	var batchTasks []*models.BatchUpgradeTask

	query := r.db.WithContext(ctx).
		Preload("UpgradeTasks").
		Preload("UpgradeTasks.Box")

	// 应用查询选项
	if options != nil {
		if options.Sort != nil && options.Sort.Field != "" {
			order := options.Sort.Field
			if options.Sort.Order == "desc" {
				order += " DESC"
			} else {
				order += " ASC"
			}
			query = query.Order(order)
		} else {
			query = query.Order("created_at DESC")
		}

		if options.Pagination != nil {
			offset := (options.Pagination.Page - 1) * options.Pagination.PageSize
			query = query.Offset(offset).Limit(options.Pagination.PageSize)
		}
	} else {
		query = query.Order("created_at DESC")
	}

	err := query.Find(&batchTasks).Error
	return batchTasks, err
}

// SearchUpgrades 搜索升级任务
func (r *upgradeRepository) SearchUpgrades(ctx context.Context, keyword string, options *QueryOptions) ([]*models.UpgradeTask, error) {
	var upgradeTasks []*models.UpgradeTask

	query := r.db.WithContext(ctx).
		Preload("Box").
		Preload("BatchUpgradeTask").
		Where("version_from LIKE ? OR version_to LIKE ? OR program_file LIKE ? OR error_message LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")

	// 应用查询选项
	query = ApplyQueryOptions[models.UpgradeTask](query, options)

	err := query.Find(&upgradeTasks).Error
	return upgradeTasks, err
}

// BulkCancel 批量取消升级任务
func (r *upgradeRepository) BulkCancel(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Model(&models.UpgradeTask{}).
		Where("id IN ? AND status IN ?", ids, []models.UpgradeStatus{
			models.UpgradeStatusPending, models.UpgradeStatusRunning,
		}).
		Updates(map[string]interface{}{
			"status":       models.UpgradeStatusCancelled,
			"completed_at": time.Now(),
		}).Error
}

// GetUpgradeReport 获取升级报告
func (r *upgradeRepository) GetUpgradeReport(ctx context.Context, startDate, endDate string) (map[string]interface{}, error) {
	report := make(map[string]interface{})

	// 基础查询条件
	baseQuery := r.db.WithContext(ctx).Model(&models.UpgradeTask{})
	if startDate != "" && endDate != "" {
		baseQuery = baseQuery.Where("created_at >= ? AND created_at <= ?", startDate, endDate)
	}

	// 升级成功率
	var completedCount, totalCount int64

	if err := baseQuery.Where("status = ?", models.UpgradeStatusCompleted).Count(&completedCount).Error; err != nil {
		return nil, err
	}

	if err := baseQuery.Where("status IN ?", []models.UpgradeStatus{
		models.UpgradeStatusCompleted, models.UpgradeStatusFailed, models.UpgradeStatusCancelled,
	}).Count(&totalCount).Error; err != nil {
		return nil, err
	}

	successRate := float64(0)
	if totalCount > 0 {
		successRate = float64(completedCount) / float64(totalCount) * 100
	}

	report["success_rate"] = successRate
	report["completed_count"] = completedCount
	report["total_count"] = totalCount

	// 平均升级时间
	type durationStats struct {
		AvgDuration float64 `json:"avg_duration"`
		MaxDuration float64 `json:"max_duration"`
		MinDuration float64 `json:"min_duration"`
	}

	var durationResult durationStats
	err := baseQuery.
		Where("started_at IS NOT NULL AND completed_at IS NOT NULL").
		Select("AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) as avg_duration, MAX(EXTRACT(EPOCH FROM (completed_at - started_at))) as max_duration, MIN(EXTRACT(EPOCH FROM (completed_at - started_at))) as min_duration").
		Find(&durationResult).Error

	if err != nil {
		return nil, err
	}
	report["duration_stats"] = durationResult

	// 版本分布统计
	type versionStats struct {
		VersionTo string `json:"version_to"`
		Count     int64  `json:"count"`
	}

	var versionResults []versionStats
	err = baseQuery.
		Select("version_to, count(*) as count").
		Group("version_to").
		Order("count DESC").
		Find(&versionResults).Error

	if err != nil {
		return nil, err
	}
	report["version_distribution"] = versionResults

	return report, nil
}

// 版本管理相关方法

// RecordVersion 记录版本信息
func (r *upgradeRepository) RecordVersion(ctx context.Context, task *models.UpgradeTask) error {
	// 开始事务
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 设置旧版本为非当前版本
		if err := tx.Model(&models.UpgradeVersion{}).
			Where("box_id = ? AND is_current = ?", task.BoxID, true).
			Update("is_current", false).Error; err != nil {
			return err
		}

		// 记录新版本
		version := &models.UpgradeVersion{
			BoxID:         task.BoxID,
			Version:       task.VersionTo,
			IsCurrent:     true,
			InstallDate:   time.Now(),
			UpgradeTaskID: &task.ID,
		}

		return tx.Create(version).Error
	})
}

// GetVersionByBoxIDAndVersion 根据盒子ID和版本号获取版本信息
func (r *upgradeRepository) GetVersionByBoxIDAndVersion(ctx context.Context, boxID uint, version string) (*models.UpgradeVersion, error) {
	var upgradeVersion models.UpgradeVersion
	err := r.db.WithContext(ctx).
		Where("box_id = ? AND version = ?", boxID, version).
		First(&upgradeVersion).Error
	if err != nil {
		return nil, err
	}
	return &upgradeVersion, nil
}

// GetCurrentVersion 获取盒子的当前版本
func (r *upgradeRepository) GetCurrentVersion(ctx context.Context, boxID uint) (*models.UpgradeVersion, error) {
	var version models.UpgradeVersion
	err := r.db.WithContext(ctx).
		Where("box_id = ? AND is_current = ?", boxID, true).
		First(&version).Error
	if err != nil {
		return nil, err
	}
	return &version, nil
}

// UpdateTasksStatusByBatch 批量更新任务状态
func (r *upgradeRepository) UpdateTasksStatusByBatch(ctx context.Context, batchID uint, status models.UpgradeStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.UpgradeTask{}).
		Where("batch_upgrade_task_id = ?", batchID).
		Update("status", status).Error
}

// GetTaskWithBox 获取包含盒子信息的任务
func (r *upgradeRepository) GetTaskWithBox(ctx context.Context, taskID uint) (*models.UpgradeTask, error) {
	var task models.UpgradeTask
	err := r.db.WithContext(ctx).
		Preload("Box").
		First(&task, taskID).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// FindUpgradeTasksWithOptions 根据条件查询升级任务（支持分页和预加载）
func (r *upgradeRepository) FindUpgradeTasksWithOptions(ctx context.Context, conditions map[string]interface{}, page, pageSize int, preloads []string) ([]*models.UpgradeTask, int64, error) {
	var tasks []*models.UpgradeTask
	var total int64

	query := r.db.WithContext(ctx).Model(&models.UpgradeTask{})

	// 应用条件
	for key, value := range conditions {
		query = query.Where(key+" = ?", value)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 应用预加载
	for _, preload := range preloads {
		query = query.Preload(preload)
	}

	// 应用分页
	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		query = query.Offset(offset).Limit(pageSize)
	}

	err := query.Order("created_at DESC").Find(&tasks).Error
	return tasks, total, err
}
