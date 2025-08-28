/*
 * @module repository/box_repository
 * @description 盒子Repository实现，提供盒子相关的数据访问操作
 * @architecture 数据访问层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow Service层 -> BoxRepository -> GORM -> 数据库
 * @rules 实现盒子的CRUD操作、状态管理、统计查询等功能
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

// boxRepository 盒子Repository实现
type boxRepository struct {
	BaseRepository[models.Box]
	db *gorm.DB
}

// NewBoxRepository 创建Box Repository实例
func NewBoxRepository(db *gorm.DB) BoxRepository {
	return &boxRepository{
		BaseRepository: newBaseRepository[models.Box](db),
		db:             db,
	}
}

// FindByIPAddress 根据IP地址查找盒子
func (r *boxRepository) FindByIPAddress(ctx context.Context, ipAddress string) (*models.Box, error) {
	var box models.Box
	// 只查找未软删除的记录 - 不使用Unscoped，GORM默认会排除软删除记录
	err := r.db.WithContext(ctx).Where("ip_address = ?", ipAddress).First(&box).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &box, nil
}

// FindByStatus 根据状态查找盒子
func (r *boxRepository) FindByStatus(ctx context.Context, status models.BoxStatus) ([]*models.Box, error) {
	var boxes []*models.Box
	err := r.db.WithContext(ctx).Where("status = ?", status).Find(&boxes).Error
	return boxes, err
}

// FindOnlineBoxes 查找在线的盒子
func (r *boxRepository) FindOnlineBoxes(ctx context.Context) ([]*models.Box, error) {
	var boxes []*models.Box

	// 在线条件：状态为online且最近5分钟内有心跳
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

	err := r.db.WithContext(ctx).
		Where("status = ? AND last_heartbeat > ?", models.BoxStatusOnline, fiveMinutesAgo).
		Find(&boxes).Error

	return boxes, err
}

// FindByLocationPattern 根据位置模式查找盒子
func (r *boxRepository) FindByLocationPattern(ctx context.Context, locationPattern string) ([]*models.Box, error) {
	var boxes []*models.Box
	err := r.db.WithContext(ctx).
		Where("location LIKE ?", "%"+locationPattern+"%").
		Find(&boxes).Error
	return boxes, err
}

// UpdateStatus 更新盒子状态
func (r *boxRepository) UpdateStatus(ctx context.Context, id uint, status models.BoxStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.Box{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateHeartbeat 更新心跳时间
func (r *boxRepository) UpdateHeartbeat(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.Box{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_heartbeat": now,
			"status":         models.BoxStatusOnline,
		}).Error
}

// UpdateResources 更新资源使用情况
func (r *boxRepository) UpdateResources(ctx context.Context, id uint, resources models.Resources) error {
	return r.db.WithContext(ctx).
		Model(&models.Box{}).
		Where("id = ?", id).
		Update("resources", resources).Error
}

// GetBoxStatistics 获取盒子统计信息
func (r *boxRepository) GetBoxStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总盒子数
	var totalCount int64
	if err := r.db.WithContext(ctx).Model(&models.Box{}).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total"] = totalCount

	// 在线盒子数
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)
	var onlineCount int64
	if err := r.db.WithContext(ctx).Model(&models.Box{}).
		Where("status = ? AND last_heartbeat > ?", models.BoxStatusOnline, fiveMinutesAgo).
		Count(&onlineCount).Error; err != nil {
		return nil, err
	}
	stats["online"] = onlineCount

	// 离线盒子数
	var offlineCount int64
	if err := r.db.WithContext(ctx).Model(&models.Box{}).
		Where("status = ? OR last_heartbeat <= ?", models.BoxStatusOffline, fiveMinutesAgo).
		Count(&offlineCount).Error; err != nil {
		return nil, err
	}
	stats["offline"] = offlineCount

	// 维护中盒子数
	var maintenanceCount int64
	if err := r.db.WithContext(ctx).Model(&models.Box{}).
		Where("status = ?", models.BoxStatusMaintenance).
		Count(&maintenanceCount).Error; err != nil {
		return nil, err
	}
	stats["maintenance"] = maintenanceCount

	// 错误状态盒子数
	var errorCount int64
	if err := r.db.WithContext(ctx).Model(&models.Box{}).
		Where("status = ?", models.BoxStatusError).
		Count(&errorCount).Error; err != nil {
		return nil, err
	}
	stats["error"] = errorCount

	// 今日新增盒子数
	today := time.Now().Truncate(24 * time.Hour)
	var todayCount int64
	if err := r.db.WithContext(ctx).Model(&models.Box{}).
		Where("created_at >= ?", today).
		Count(&todayCount).Error; err != nil {
		return nil, err
	}
	stats["today_added"] = todayCount

	return stats, nil
}

// GetBoxStatusDistribution 获取盒子状态分布
func (r *boxRepository) GetBoxStatusDistribution(ctx context.Context) (map[models.BoxStatus]int64, error) {
	distribution := make(map[models.BoxStatus]int64)

	type statusCount struct {
		Status models.BoxStatus `json:"status"`
		Count  int64            `json:"count"`
	}

	var results []statusCount
	err := r.db.WithContext(ctx).
		Model(&models.Box{}).
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

// CountOnlineBoxes 统计在线盒子数量
func (r *boxRepository) CountOnlineBoxes() (int64, error) {
	var count int64
	err := r.db.Model(&models.Box{}).
		Where("status = ? AND deleted_at IS NULL", models.BoxStatusOnline).
		Count(&count).Error
	return count, err
}

// GetBoxesWithRelations 获取带关联关系的盒子列表
func (r *boxRepository) GetBoxesWithRelations(ctx context.Context, options *QueryOptions) ([]*models.Box, error) {
	var boxes []*models.Box

	query := r.db.WithContext(ctx).
		Preload("Tasks").
		Preload("Models").
		Preload("Upgrades").
		Preload("Heartbeats", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(10) // 只加载最近10条心跳记录
		})

	// 应用查询选项
	query = ApplyQueryOptions[models.Box](query, options)

	err := query.Find(&boxes).Error
	return boxes, err
}

// GetBoxByIDWithRelations 根据ID获取带关联关系的盒子
func (r *boxRepository) GetBoxByIDWithRelations(ctx context.Context, id uint) (*models.Box, error) {
	var box models.Box

	err := r.db.WithContext(ctx).
		Preload("Tasks").
		Preload("Models").
		Preload("Upgrades", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(20) // 最近20个升级任务
		}).
		Preload("Heartbeats", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(100) // 最近100条心跳记录
		}).
		First(&box, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &box, nil
}

// BulkUpdateStatus 批量更新盒子状态
func (r *boxRepository) BulkUpdateStatus(ctx context.Context, ids []uint, status models.BoxStatus) error {
	if len(ids) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Model(&models.Box{}).
		Where("id IN ?", ids).
		Update("status", status).Error
}

// GetBoxesByLocation 根据位置获取盒子列表
func (r *boxRepository) GetBoxesByLocation(ctx context.Context, location string) ([]*models.Box, error) {
	var boxes []*models.Box
	err := r.db.WithContext(ctx).
		Where("location = ?", location).
		Order("name ASC").
		Find(&boxes).Error
	return boxes, err
}

// GetRecentlyAddedBoxes 获取最近添加的盒子
func (r *boxRepository) GetRecentlyAddedBoxes(ctx context.Context, limit int) ([]*models.Box, error) {
	var boxes []*models.Box
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&boxes).Error
	return boxes, err
}

// SearchBoxes 搜索盒子
func (r *boxRepository) SearchBoxes(ctx context.Context, keyword string, options *QueryOptions) ([]*models.Box, error) {
	var boxes []*models.Box

	query := r.db.WithContext(ctx).
		Where("name LIKE ? OR ip_address LIKE ? OR location LIKE ? OR description LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")

	// 应用查询选项
	query = ApplyQueryOptions[models.Box](query, options)

	err := query.Find(&boxes).Error
	return boxes, err
}

// IsIPAddressExists 检查IP地址是否已存在
func (r *boxRepository) IsIPAddressExists(ctx context.Context, ipAddress string, excludeID ...uint) (bool, error) {
	query := r.db.WithContext(ctx).Model(&models.Box{}).Where("ip_address = ?", ipAddress)

	// 排除指定ID（用于更新时检查）
	if len(excludeID) > 0 && excludeID[0] > 0 {
		query = query.Where("id != ?", excludeID[0])
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}
