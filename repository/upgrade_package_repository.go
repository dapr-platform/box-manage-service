/*
 * @module repository/upgrade_package_repository
 * @description 升级包Repository实现，提供数据访问层功能
 * @architecture 数据访问层
 * @documentReference REQ-001: 盒子管理功能 - 升级文件管理
 * @stateFlow 业务层 -> Repository接口 -> 具体实现 -> 数据库
 * @rules 遵循GORM最佳实践，实现CRUD操作、分页、条件查询等通用功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// upgradePackageRepository 升级包Repository实现
type upgradePackageRepository struct {
	db *gorm.DB
}

// NewUpgradePackageRepository 创建升级包Repository实例
func NewUpgradePackageRepository(db *gorm.DB) UpgradePackageRepository {
	return &upgradePackageRepository{
		db: db,
	}
}

// 实现 BaseRepository 接口

// Create 创建升级包
func (r *upgradePackageRepository) Create(ctx context.Context, entity *models.UpgradePackage) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// CreateBatch 批量创建升级包
func (r *upgradePackageRepository) CreateBatch(ctx context.Context, entities []*models.UpgradePackage) error {
	return r.db.WithContext(ctx).Create(entities).Error
}

// GetByID 根据ID获取升级包
func (r *upgradePackageRepository) GetByID(ctx context.Context, id uint) (*models.UpgradePackage, error) {
	var pkg models.UpgradePackage
	err := r.db.WithContext(ctx).First(&pkg, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &pkg, nil
}

// Update 更新升级包
func (r *upgradePackageRepository) Update(ctx context.Context, entity *models.UpgradePackage) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

// Delete 硬删除升级包
func (r *upgradePackageRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.UpgradePackage{}, id).Error
}

// SoftDelete 软删除升级包
func (r *upgradePackageRepository) SoftDelete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.UpgradePackage{}).Where("id = ?", id).Update("deleted_at", time.Now()).Error
}

// DeleteBatch 批量删除升级包
func (r *upgradePackageRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	return r.db.WithContext(ctx).Delete(&models.UpgradePackage{}, ids).Error
}

// UpdateBatch 批量更新升级包
func (r *upgradePackageRepository) UpdateBatch(ctx context.Context, entities []*models.UpgradePackage) error {
	return r.db.WithContext(ctx).Save(entities).Error
}

// Find 根据条件查找升级包
func (r *upgradePackageRepository) Find(ctx context.Context, conditions map[string]interface{}) ([]*models.UpgradePackage, error) {
	var packages []*models.UpgradePackage
	query := r.db.WithContext(ctx)
	for key, value := range conditions {
		query = query.Where(key+" = ?", value)
	}
	err := query.Find(&packages).Error
	return packages, err
}

// FindAll 查找所有升级包
func (r *upgradePackageRepository) FindAll(ctx context.Context) ([]*models.UpgradePackage, error) {
	var packages []*models.UpgradePackage
	err := r.db.WithContext(ctx).Find(&packages).Error
	return packages, err
}

// FindWithPagination 分页查询升级包
func (r *upgradePackageRepository) FindWithPagination(ctx context.Context, conditions map[string]interface{}, page, pageSize int) ([]*models.UpgradePackage, int64, error) {
	var packages []*models.UpgradePackage
	var total int64

	query := r.db.WithContext(ctx).Model(&models.UpgradePackage{})

	// 应用条件
	for key, value := range conditions {
		query = query.Where(key+" = ?", value)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Find(&packages).Error; err != nil {
		return nil, 0, err
	}

	return packages, total, nil
}

// Count 计算总数
func (r *upgradePackageRepository) Count(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.UpgradePackage{})
	for key, value := range conditions {
		query = query.Where(key+" = ?", value)
	}
	err := query.Count(&count).Error
	return count, err
}

// CountByCondition 根据条件计算数量
func (r *upgradePackageRepository) CountByCondition(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.UpgradePackage{})
	for key, value := range conditions {
		query = query.Where(key+" = ?", value)
	}
	err := query.Count(&count).Error
	return count, err
}

// Exists 检查是否存在
func (r *upgradePackageRepository) Exists(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.UpgradePackage{})
	for key, value := range conditions {
		query = query.Where(key+" = ?", value)
	}
	err := query.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByNameAndVersion 根据名称和版本查找升级包
func (r *upgradePackageRepository) FindByNameAndVersion(ctx context.Context, name, version string) (*models.UpgradePackage, error) {
	var pkg models.UpgradePackage
	err := r.db.WithContext(ctx).Where("name = ? AND version = ?", name, version).First(&pkg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &pkg, nil
}

// FindByVersion 根据版本查找升级包
func (r *upgradePackageRepository) FindByVersion(ctx context.Context, version string) ([]*models.UpgradePackage, error) {
	var packages []*models.UpgradePackage
	err := r.db.WithContext(ctx).Where("version = ?", version).Find(&packages).Error
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// FindByType 根据类型查找升级包
func (r *upgradePackageRepository) FindByType(ctx context.Context, packageType models.UpgradePackageType) ([]*models.UpgradePackage, error) {
	var packages []*models.UpgradePackage
	err := r.db.WithContext(ctx).Where("type = ?", packageType).Find(&packages).Error
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// FindByStatus 根据状态查找升级包
func (r *upgradePackageRepository) FindByStatus(ctx context.Context, status models.PackageStatus) ([]*models.UpgradePackage, error) {
	var packages []*models.UpgradePackage
	err := r.db.WithContext(ctx).Where("status = ?", status).Find(&packages).Error
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// FindAvailable 获取可用的升级包列表(status=ready且未删除)
func (r *upgradePackageRepository) FindAvailable(ctx context.Context) ([]*models.UpgradePackage, error) {
	var packages []*models.UpgradePackage
	err := r.db.WithContext(ctx).
		Where("status = ? AND deleted_at IS NULL", models.PackageStatusReady).
		Order("created_at DESC").
		Find(&packages).Error
	if err != nil {
		return nil, err
	}
	return packages, nil
}

// FindLatestByType 获取最新版本的升级包
func (r *upgradePackageRepository) FindLatestByType(ctx context.Context, packageType models.UpgradePackageType) (*models.UpgradePackage, error) {
	var pkg models.UpgradePackage
	err := r.db.WithContext(ctx).
		Where("type = ? AND status = ? AND deleted_at IS NULL", packageType, models.PackageStatusReady).
		Order("created_at DESC").
		First(&pkg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &pkg, nil
}

// VersionExists 检查版本是否存在
func (r *upgradePackageRepository) VersionExists(ctx context.Context, version string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.UpgradePackage{}).
		Where("version = ? AND deleted_at IS NULL", version).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetStatistics 获取升级包统计信息
func (r *upgradePackageRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数量
	var totalCount int64
	err := r.db.WithContext(ctx).Model(&models.UpgradePackage{}).Count(&totalCount).Error
	if err != nil {
		return nil, err
	}
	stats["total"] = totalCount

	// 按状态统计
	var statusStats []struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	err = r.db.WithContext(ctx).
		Model(&models.UpgradePackage{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&statusStats).Error
	if err != nil {
		return nil, err
	}
	stats["by_status"] = statusStats

	// 按类型统计
	var typeStats []struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	err = r.db.WithContext(ctx).
		Model(&models.UpgradePackage{}).
		Select("type, count(*) as count").
		Group("type").
		Find(&typeStats).Error
	if err != nil {
		return nil, err
	}
	stats["by_type"] = typeStats

	// 可用包数量
	var readyCount int64
	err = r.db.WithContext(ctx).
		Model(&models.UpgradePackage{}).
		Where("status = ? AND deleted_at IS NULL", models.PackageStatusReady).
		Count(&readyCount).Error
	if err != nil {
		return nil, err
	}
	stats["ready_count"] = readyCount

	// 总下载次数
	var totalDownloads int64
	err = r.db.WithContext(ctx).
		Model(&models.UpgradePackage{}).
		Select("COALESCE(SUM(download_count), 0)").
		Scan(&totalDownloads).Error
	if err != nil {
		return nil, err
	}
	stats["total_downloads"] = totalDownloads

	// 总升级次数
	var totalUpgrades int64
	err = r.db.WithContext(ctx).
		Model(&models.UpgradePackage{}).
		Select("COALESCE(SUM(upgrade_count), 0)").
		Scan(&totalUpgrades).Error
	if err != nil {
		return nil, err
	}
	stats["total_upgrades"] = totalUpgrades

	return stats, nil
}

// BatchUpdateStatus 批量更新状态
func (r *upgradePackageRepository) BatchUpdateStatus(ctx context.Context, ids []uint, status models.PackageStatus) error {
	if len(ids) == 0 {
		return errors.New("ID列表不能为空")
	}

	return r.db.WithContext(ctx).
		Model(&models.UpgradePackage{}).
		Where("id IN ?", ids).
		Update("status", status).Error
}
