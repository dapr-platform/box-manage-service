/*
 * @module repository/system_log_repository
 * @description 系统日志数据访问层
 * @architecture Repository模式 - 数据访问层
 * @documentReference DESIGN-000.md
 * @stateFlow 数据查询 -> 条件过滤 -> 结果返回
 * @rules 提供系统日志的CRUD操作和查询功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// SystemLogRepository 系统日志Repository接口
type SystemLogRepository interface {
	// 基础CRUD操作
	Create(ctx context.Context, log *models.SystemLog) error
	GetByID(ctx context.Context, id uint) (*models.SystemLog, error)
	Update(ctx context.Context, log *models.SystemLog) error
	Delete(ctx context.Context, id uint) error

	// 查询操作
	Find(ctx context.Context, filters map[string]interface{}) ([]*models.SystemLog, error)
	FindWithPagination(ctx context.Context, filters map[string]interface{}, pagination *PaginationOptions) ([]*models.SystemLog, int64, error)
	FindByLevel(ctx context.Context, level models.LogLevel, limit int) ([]*models.SystemLog, error)
	FindBySource(ctx context.Context, source string, limit int) ([]*models.SystemLog, error)
	FindByTimeRange(ctx context.Context, startTime, endTime time.Time, limit int) ([]*models.SystemLog, error)
	FindByUserID(ctx context.Context, userID uint, limit int) ([]*models.SystemLog, error)
	FindByRequestID(ctx context.Context, requestID string) ([]*models.SystemLog, error)

	// 统计操作
	CountByLevel(ctx context.Context, level models.LogLevel) (int64, error)
	CountBySource(ctx context.Context, source string) (int64, error)
	CountByTimeRange(ctx context.Context, startTime, endTime time.Time) (int64, error)
	GetStatistics(ctx context.Context, startTime, endTime time.Time) (map[string]interface{}, error)

	// 清理操作
	DeleteOlderThan(ctx context.Context, olderThan time.Time) (int64, error)
	DeleteByLevel(ctx context.Context, level models.LogLevel, olderThan time.Time) (int64, error)
}

// systemLogRepository 系统日志Repository实现
type systemLogRepository struct {
	db *gorm.DB
}

// NewSystemLogRepository 创建系统日志Repository实例
func NewSystemLogRepository(db *gorm.DB) SystemLogRepository {
	return &systemLogRepository{
		db: db,
	}
}

// Create 创建系统日志
func (r *systemLogRepository) Create(ctx context.Context, log *models.SystemLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByID 根据ID获取系统日志
func (r *systemLogRepository) GetByID(ctx context.Context, id uint) (*models.SystemLog, error) {
	var log models.SystemLog
	err := r.db.WithContext(ctx).First(&log, id).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// Update 更新系统日志
func (r *systemLogRepository) Update(ctx context.Context, log *models.SystemLog) error {
	return r.db.WithContext(ctx).Save(log).Error
}

// Delete 删除系统日志
func (r *systemLogRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.SystemLog{}, id).Error
}

// Find 根据条件查找系统日志
func (r *systemLogRepository) Find(ctx context.Context, filters map[string]interface{}) ([]*models.SystemLog, error) {
	var logs []*models.SystemLog
	query := r.db.WithContext(ctx).Model(&models.SystemLog{})

	// 应用过滤条件
	query = r.applyFilters(query, filters)

	// 按创建时间倒序排列
	query = query.Order("created_at DESC")

	err := query.Find(&logs).Error
	return logs, err
}

// FindWithPagination 分页查找系统日志
func (r *systemLogRepository) FindWithPagination(ctx context.Context, filters map[string]interface{}, pagination *PaginationOptions) ([]*models.SystemLog, int64, error) {
	var logs []*models.SystemLog
	var total int64

	query := r.db.WithContext(ctx).Model(&models.SystemLog{})

	// 应用过滤条件
	query = r.applyFilters(query, filters)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 应用分页
	if pagination != nil {
		offset := (pagination.Page - 1) * pagination.PageSize
		query = query.Offset(offset).Limit(pagination.PageSize)
	}

	// 按创建时间倒序排列
	query = query.Order("created_at DESC")

	err := query.Find(&logs).Error
	return logs, total, err
}

// FindByLevel 根据日志级别查找
func (r *systemLogRepository) FindByLevel(ctx context.Context, level models.LogLevel, limit int) ([]*models.SystemLog, error) {
	var logs []*models.SystemLog
	query := r.db.WithContext(ctx).Where("level = ?", level).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	return logs, err
}

// FindBySource 根据来源查找
func (r *systemLogRepository) FindBySource(ctx context.Context, source string, limit int) ([]*models.SystemLog, error) {
	var logs []*models.SystemLog
	query := r.db.WithContext(ctx).Where("source = ?", source).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	return logs, err
}

// FindByTimeRange 根据时间范围查找
func (r *systemLogRepository) FindByTimeRange(ctx context.Context, startTime, endTime time.Time, limit int) ([]*models.SystemLog, error) {
	var logs []*models.SystemLog
	query := r.db.WithContext(ctx).Where("created_at BETWEEN ? AND ?", startTime, endTime).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	return logs, err
}

// FindByUserID 根据用户ID查找
func (r *systemLogRepository) FindByUserID(ctx context.Context, userID uint, limit int) ([]*models.SystemLog, error) {
	var logs []*models.SystemLog
	query := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&logs).Error
	return logs, err
}

// FindByRequestID 根据请求ID查找
func (r *systemLogRepository) FindByRequestID(ctx context.Context, requestID string) ([]*models.SystemLog, error) {
	var logs []*models.SystemLog
	err := r.db.WithContext(ctx).Where("request_id = ?", requestID).Order("created_at ASC").Find(&logs).Error
	return logs, err
}

// CountByLevel 根据级别统计数量
func (r *systemLogRepository) CountByLevel(ctx context.Context, level models.LogLevel) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.SystemLog{}).Where("level = ?", level).Count(&count).Error
	return count, err
}

// CountBySource 根据来源统计数量
func (r *systemLogRepository) CountBySource(ctx context.Context, source string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.SystemLog{}).Where("source = ?", source).Count(&count).Error
	return count, err
}

// CountByTimeRange 根据时间范围统计数量
func (r *systemLogRepository) CountByTimeRange(ctx context.Context, startTime, endTime time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.SystemLog{}).Where("created_at BETWEEN ? AND ?", startTime, endTime).Count(&count).Error
	return count, err
}

// GetStatistics 获取日志统计信息
func (r *systemLogRepository) GetStatistics(ctx context.Context, startTime, endTime time.Time) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 按级别统计
	var levelStats []struct {
		Level models.LogLevel `json:"level"`
		Count int64           `json:"count"`
	}

	err := r.db.WithContext(ctx).Model(&models.SystemLog{}).
		Select("level, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Group("level").
		Find(&levelStats).Error
	if err != nil {
		return nil, err
	}
	stats["by_level"] = levelStats

	// 按来源统计
	var sourceStats []struct {
		Source string `json:"source"`
		Count  int64  `json:"count"`
	}

	err = r.db.WithContext(ctx).Model(&models.SystemLog{}).
		Select("source, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Group("source").
		Order("count DESC").
		Limit(20). // 只返回前20个
		Find(&sourceStats).Error
	if err != nil {
		return nil, err
	}
	stats["by_source"] = sourceStats

	// 按小时统计
	var hourlyStats []struct {
		Hour  string `json:"hour"`
		Count int64  `json:"count"`
	}

	err = r.db.WithContext(ctx).Model(&models.SystemLog{}).
		Select("DATE_TRUNC('hour', created_at) as hour, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Group("hour").
		Order("hour").
		Find(&hourlyStats).Error
	if err != nil {
		return nil, err
	}
	stats["by_hour"] = hourlyStats

	// 总数统计
	var total int64
	err = r.db.WithContext(ctx).Model(&models.SystemLog{}).
		Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Count(&total).Error
	if err != nil {
		return nil, err
	}
	stats["total"] = total

	return stats, nil
}

// DeleteOlderThan 删除指定时间之前的日志
func (r *systemLogRepository) DeleteOlderThan(ctx context.Context, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("created_at < ?", olderThan).Delete(&models.SystemLog{})
	return result.RowsAffected, result.Error
}

// DeleteByLevel 删除指定级别和时间之前的日志
func (r *systemLogRepository) DeleteByLevel(ctx context.Context, level models.LogLevel, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("level = ? AND created_at < ?", level, olderThan).Delete(&models.SystemLog{})
	return result.RowsAffected, result.Error
}

// applyFilters 应用过滤条件
func (r *systemLogRepository) applyFilters(query *gorm.DB, filters map[string]interface{}) *gorm.DB {
	if level, ok := filters["level"]; ok {
		query = query.Where("level = ?", level)
	}

	if source, ok := filters["source"]; ok {
		query = query.Where("source = ?", source)
	}

	if sourceID, ok := filters["source_id"]; ok {
		query = query.Where("source_id = ?", sourceID)
	}

	if userID, ok := filters["user_id"]; ok {
		query = query.Where("user_id = ?", userID)
	}

	if requestID, ok := filters["request_id"]; ok {
		query = query.Where("request_id = ?", requestID)
	}

	if startTime, ok := filters["start_time"]; ok {
		query = query.Where("created_at >= ?", startTime)
	}

	if endTime, ok := filters["end_time"]; ok {
		query = query.Where("created_at <= ?", endTime)
	}

	if keyword, ok := filters["keyword"]; ok {
		keyword := "%" + keyword.(string) + "%"
		query = query.Where("title LIKE ? OR message LIKE ?", keyword, keyword)
	}

	return query
}
