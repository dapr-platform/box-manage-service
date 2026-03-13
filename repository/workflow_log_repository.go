/*
 * @module repository/workflow_log_repository
 * @description 工作流日志Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> WorkflowLogRepository -> GORM -> 数据库
 * @rules 实现工作流日志的CRUD操作、查询和清理功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.9节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// WorkflowLogRepository 工作流日志Repository接口
type WorkflowLogRepository interface {
	BaseRepository[models.WorkflowLog]

	// 基础查询
	FindByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint, limit int) ([]*models.WorkflowLog, error)
	FindByNodeInstanceID(ctx context.Context, nodeInstanceID uint, limit int) ([]*models.WorkflowLog, error)
	FindByLevel(ctx context.Context, level models.LogLevel) ([]*models.WorkflowLog, error)
	FindByType(ctx context.Context, logType models.LogType) ([]*models.WorkflowLog, error)

	// 时间范围查询
	FindByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*models.WorkflowLog, error)
	FindByWorkflowInstanceIDAndTimeRange(ctx context.Context, workflowInstanceID uint, startTime, endTime time.Time) ([]*models.WorkflowLog, error)

	// 清理操作
	CleanupOldLogs(ctx context.Context, olderThan time.Time) (int64, error)
	CleanupByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint, keepLatest int) error
}

// workflowLogRepository 工作流日志Repository实现
type workflowLogRepository struct {
	BaseRepository[models.WorkflowLog]
	db *gorm.DB
}

// NewWorkflowLogRepository 创建WorkflowLog Repository实例
func NewWorkflowLogRepository(db *gorm.DB) WorkflowLogRepository {
	return &workflowLogRepository{
		BaseRepository: newBaseRepository[models.WorkflowLog](db),
		db:             db,
	}
}

// FindByWorkflowInstanceID 根据工作流实例ID查找日志
func (r *workflowLogRepository) FindByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint, limit int) ([]*models.WorkflowLog, error) {
	var logs []*models.WorkflowLog
	query := r.db.WithContext(ctx).
		Where("workflow_instance_id = ?", workflowInstanceID).
		Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	return logs, err
}

// FindByNodeInstanceID 根据节点实例ID查找日志
func (r *workflowLogRepository) FindByNodeInstanceID(ctx context.Context, nodeInstanceID uint, limit int) ([]*models.WorkflowLog, error) {
	var logs []*models.WorkflowLog
	query := r.db.WithContext(ctx).
		Where("node_instance_id = ?", nodeInstanceID).
		Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	return logs, err
}

// FindByLevel 根据日志级别查找日志
func (r *workflowLogRepository) FindByLevel(ctx context.Context, level models.LogLevel) ([]*models.WorkflowLog, error) {
	var logs []*models.WorkflowLog
	err := r.db.WithContext(ctx).
		Where("level = ?", level).
		Order("timestamp DESC").
		Find(&logs).Error
	return logs, err
}

// FindByType 根据日志类型查找日志
func (r *workflowLogRepository) FindByType(ctx context.Context, logType models.LogType) ([]*models.WorkflowLog, error) {
	var logs []*models.WorkflowLog
	err := r.db.WithContext(ctx).
		Where("type = ?", logType).
		Order("timestamp DESC").
		Find(&logs).Error
	return logs, err
}

// FindByTimeRange 根据时间范围查找日志
func (r *workflowLogRepository) FindByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*models.WorkflowLog, error) {
	var logs []*models.WorkflowLog
	err := r.db.WithContext(ctx).
		Where("timestamp BETWEEN ? AND ?", startTime, endTime).
		Order("timestamp DESC").
		Find(&logs).Error
	return logs, err
}

// FindByWorkflowInstanceIDAndTimeRange 根据工作流实例ID和时间范围查找日志
func (r *workflowLogRepository) FindByWorkflowInstanceIDAndTimeRange(ctx context.Context, workflowInstanceID uint, startTime, endTime time.Time) ([]*models.WorkflowLog, error) {
	var logs []*models.WorkflowLog
	err := r.db.WithContext(ctx).
		Where("workflow_instance_id = ? AND timestamp BETWEEN ? AND ?", workflowInstanceID, startTime, endTime).
		Order("timestamp DESC").
		Find(&logs).Error
	return logs, err
}

// CleanupOldLogs 清理旧日志
func (r *workflowLogRepository) CleanupOldLogs(ctx context.Context, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("timestamp < ?", olderThan).
		Delete(&models.WorkflowLog{})
	return result.RowsAffected, result.Error
}

// CleanupByWorkflowInstanceID 清理工作流实例的日志，保留最新的N条
func (r *workflowLogRepository) CleanupByWorkflowInstanceID(ctx context.Context, workflowInstanceID uint, keepLatest int) error {
	// 查找要保留的日志ID
	var keepIDs []uint
	err := r.db.WithContext(ctx).
		Model(&models.WorkflowLog{}).
		Where("workflow_instance_id = ?", workflowInstanceID).
		Order("timestamp DESC").
		Limit(keepLatest).
		Pluck("id", &keepIDs).Error

	if err != nil {
		return err
	}

	if len(keepIDs) == 0 {
		return nil
	}

	// 删除不在保留列表中的日志
	return r.db.WithContext(ctx).
		Where("workflow_instance_id = ? AND id NOT IN ?", workflowInstanceID, keepIDs).
		Delete(&models.WorkflowLog{}).Error
}
