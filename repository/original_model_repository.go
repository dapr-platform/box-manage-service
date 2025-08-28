/*
 * @module repository/original_model_repository
 * @description 原始模型Repository实现，提供原始模型相关的数据访问操作
 * @architecture 数据访问层
 * @documentReference REQ-002: 原始模型管理
 * @stateFlow Service层 -> OriginalModelRepository -> GORM -> 数据库
 * @rules 实现原始模型的CRUD操作、版本管理、搜索、统计等功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs REQ-002.md, DESIGN-003.md
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// originalModelRepository 原始模型Repository实现
type originalModelRepository struct {
	BaseRepository[models.OriginalModel]
	db *gorm.DB
}

// NewOriginalModelRepository 创建原始模型Repository实例
func NewOriginalModelRepository(db *gorm.DB) OriginalModelRepository {
	return &originalModelRepository{
		BaseRepository: newBaseRepository[models.OriginalModel](db),
		db:             db,
	}
}

// FindByName 根据名称查找模型
func (r *originalModelRepository) FindByName(ctx context.Context, name string) ([]*models.OriginalModel, error) {
	var modelList []*models.OriginalModel
	err := r.db.WithContext(ctx).Where("name = ?", name).Find(&modelList).Error
	return modelList, err
}

// FindByType 根据类型查找模型
func (r *originalModelRepository) FindByType(ctx context.Context, modelType models.OriginalModelType) ([]*models.OriginalModel, error) {
	var modelList []*models.OriginalModel
	err := r.db.WithContext(ctx).Where("model_type = ?", modelType).Find(&modelList).Error
	return modelList, err
}

// FindByStatus 根据状态查找模型
func (r *originalModelRepository) FindByStatus(ctx context.Context, status models.OriginalModelStatus) ([]*models.OriginalModel, error) {
	var modelList []*models.OriginalModel
	err := r.db.WithContext(ctx).Where("status = ?", status).Find(&modelList).Error
	return modelList, err
}

// FindByUserID 根据用户ID查找模型
func (r *originalModelRepository) FindByUserID(ctx context.Context, userID uint) ([]*models.OriginalModel, error) {
	var modelList []*models.OriginalModel
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&modelList).Error
	return modelList, err
}

// FindByMD5 根据MD5查找模型
func (r *originalModelRepository) FindByMD5(ctx context.Context, md5 string) (*models.OriginalModel, error) {
	var model models.OriginalModel
	err := r.db.WithContext(ctx).Where("file_md5 = ?", md5).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

// SearchModels 搜索模型
func (r *originalModelRepository) SearchModels(ctx context.Context, keyword string, options *QueryOptions) ([]*models.OriginalModel, error) {
	var modelList []*models.OriginalModel

	query := r.db.WithContext(ctx).
		Where("name LIKE ? OR description LIKE ? OR tags LIKE ? OR author LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")

	// 应用查询选项
	query = ApplyQueryOptions[models.OriginalModel](query, options)

	err := query.Find(&modelList).Error
	return modelList, err
}

// FindByTags 根据标签查找模型
func (r *originalModelRepository) FindByTags(ctx context.Context, tags []string) ([]*models.OriginalModel, error) {
	var modelList []*models.OriginalModel

	query := r.db.WithContext(ctx)

	// 构建标签查询条件
	for i, tag := range tags {
		if i == 0 {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		} else {
			query = query.Or("tags LIKE ?", "%"+tag+"%")
		}
	}

	err := query.Find(&modelList).Error
	return modelList, err
}

// FindPublicModels 查找就绪状态的模型（原公开模型逻辑已简化）
func (r *originalModelRepository) FindPublicModels(ctx context.Context, options *QueryOptions) ([]*models.OriginalModel, error) {
	var modelList []*models.OriginalModel

	query := r.db.WithContext(ctx).
		Where("status = ?", models.OriginalModelStatusReady)

	// 应用查询选项
	query = ApplyQueryOptions[models.OriginalModel](query, options)

	err := query.Find(&modelList).Error
	return modelList, err
}

// FindByNameAndVersion 根据名称和版本查找模型
func (r *originalModelRepository) FindByNameAndVersion(ctx context.Context, name, version string) (*models.OriginalModel, error) {
	var model models.OriginalModel
	err := r.db.WithContext(ctx).
		Where("name = ? AND version = ?", name, version).
		First(&model).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &model, nil
}

// GetModelVersions 获取模型的所有版本
func (r *originalModelRepository) GetModelVersions(ctx context.Context, name string) ([]*models.OriginalModel, error) {
	var modelList []*models.OriginalModel

	err := r.db.WithContext(ctx).
		Where("name = ?", name).
		Order("created_at DESC").
		Find(&modelList).Error

	return modelList, err
}

// GetLatestVersion 获取模型的最新版本
func (r *originalModelRepository) GetLatestVersion(ctx context.Context, name string) (*models.OriginalModel, error) {
	var model models.OriginalModel
	err := r.db.WithContext(ctx).
		Where("name = ?", name).
		Order("created_at DESC").
		First(&model).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &model, nil
}

// UpdateStatus 更新模型状态
func (r *originalModelRepository) UpdateStatus(ctx context.Context, id uint, status models.OriginalModelStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateUploadProgress 更新上传进度
func (r *originalModelRepository) UpdateUploadProgress(ctx context.Context, id uint, progress int) error {
	return r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("id = ?", id).
		Update("upload_progress", progress).Error
}

// MarkAsValidated 标记为已验证
func (r *originalModelRepository) MarkAsValidated(ctx context.Context, id uint, validationMsg string) error {
	return r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_validated":   true,
			"status":         models.OriginalModelStatusReady,
			"validation_msg": validationMsg,
		}).Error
}

// MarkAsInvalid 标记为无效
func (r *originalModelRepository) MarkAsInvalid(ctx context.Context, id uint, validationMsg string) error {
	return r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_validated":   false,
			"status":         models.OriginalModelStatusFailed,
			"validation_msg": validationMsg,
		}).Error
}

// UpdateStorageClass 更新存储类别
func (r *originalModelRepository) UpdateStorageClass(ctx context.Context, id uint, storageClass string) error {
	return r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("id = ?", id).
		Update("storage_class", storageClass).Error
}

// UpdateLastAccessed 更新最后访问时间
func (r *originalModelRepository) UpdateLastAccessed(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("id = ?", id).
		Update("last_accessed", time.Now()).Error
}

// IncrementDownloadCount 增加下载次数
func (r *originalModelRepository) IncrementDownloadCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("id = ?", id).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error
}

// GetModelStatistics 获取模型统计信息
func (r *originalModelRepository) GetModelStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总模型数
	var totalCount int64
	if err := r.db.WithContext(ctx).Model(&models.OriginalModel{}).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total"] = totalCount

	// 按状态统计
	statusStats := make(map[string]int64)
	type statusCount struct {
		Status models.OriginalModelStatus `json:"status"`
		Count  int64                      `json:"count"`
	}

	var statusResults []statusCount
	err := r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
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

	// 按类型统计
	typeStats := make(map[string]int64)
	type typeCount struct {
		ModelType models.OriginalModelType `json:"model_type"`
		Count     int64                    `json:"count"`
	}

	var typeResults []typeCount
	err = r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Select("model_type, count(*) as count").
		Group("model_type").
		Find(&typeResults).Error

	if err != nil {
		return nil, err
	}

	for _, result := range typeResults {
		typeStats[string(result.ModelType)] = result.Count
	}
	stats["by_type"] = typeStats

	// 就绪状态模型数（原公开模型统计已简化）
	var readyCount int64
	if err := r.db.WithContext(ctx).Model(&models.OriginalModel{}).
		Where("status = ?", models.OriginalModelStatusReady).
		Count(&readyCount).Error; err != nil {
		return nil, err
	}
	stats["ready"] = readyCount

	// 总下载次数
	type downloadCount struct {
		TotalDownloads int64 `json:"total_downloads"`
	}
	var downloads downloadCount
	err = r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Select("SUM(download_count) as total_downloads").
		Scan(&downloads).Error
	if err != nil {
		return nil, err
	}
	stats["total_downloads"] = downloads.TotalDownloads

	return stats, nil
}

// GetStorageStatistics 获取存储统计信息
func (r *originalModelRepository) GetStorageStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总存储大小
	type storageSize struct {
		TotalSize int64 `json:"total_size"`
	}
	var size storageSize
	err := r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Select("SUM(file_size) as total_size").
		Scan(&size).Error
	if err != nil {
		return nil, err
	}
	stats["total_size"] = size.TotalSize

	// 按存储类别统计
	storageClassStats := make(map[string]map[string]int64)
	type storageClassCount struct {
		StorageClass string `json:"storage_class"`
		Count        int64  `json:"count"`
		TotalSize    int64  `json:"total_size"`
	}

	var storageResults []storageClassCount
	err = r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Select("storage_class, count(*) as count, SUM(file_size) as total_size").
		Group("storage_class").
		Find(&storageResults).Error

	if err != nil {
		return nil, err
	}

	for _, result := range storageResults {
		storageClassStats[result.StorageClass] = map[string]int64{
			"count":      result.Count,
			"total_size": result.TotalSize,
		}
	}
	stats["by_storage_class"] = storageClassStats

	// 按类型存储统计
	typeStorageStats := make(map[string]map[string]int64)
	type typeStorageCount struct {
		ModelType string `json:"model_type"`
		Count     int64  `json:"count"`
		TotalSize int64  `json:"total_size"`
	}

	var typeStorageResults []typeStorageCount
	err = r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Select("model_type, count(*) as count, SUM(file_size) as total_size").
		Group("model_type").
		Find(&typeStorageResults).Error

	if err != nil {
		return nil, err
	}

	for _, result := range typeStorageResults {
		typeStorageStats[result.ModelType] = map[string]int64{
			"count":      result.Count,
			"total_size": result.TotalSize,
		}
	}
	stats["by_type"] = typeStorageStats

	return stats, nil
}

// GetUserModelStatistics 获取用户模型统计信息
func (r *originalModelRepository) GetUserModelStatistics(ctx context.Context, userID uint) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 用户模型总数
	var totalCount int64
	if err := r.db.WithContext(ctx).Model(&models.OriginalModel{}).
		Where("user_id = ?", userID).
		Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total"] = totalCount

	// 用户存储使用量
	type userStorage struct {
		TotalSize int64 `json:"total_size"`
	}
	var storage userStorage
	err := r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("user_id = ?", userID).
		Select("SUM(file_size) as total_size").
		Scan(&storage).Error
	if err != nil {
		return nil, err
	}
	stats["total_size"] = storage.TotalSize

	// 按状态统计
	statusStats := make(map[string]int64)
	type statusCount struct {
		Status models.OriginalModelStatus `json:"status"`
		Count  int64                      `json:"count"`
	}

	var statusResults []statusCount
	err = r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("user_id = ?", userID).
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

	return stats, nil
}

// BulkUpdateStatus 批量更新模型状态
func (r *originalModelRepository) BulkUpdateStatus(ctx context.Context, ids []uint, status models.OriginalModelStatus) error {
	if len(ids) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("id IN ?", ids).
		Update("status", status).Error
}

// BulkDelete 批量删除模型
func (r *originalModelRepository) BulkDelete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Delete(&models.OriginalModel{}).Error
}

// CleanupFailedUploads 清理失败的上传
func (r *originalModelRepository) CleanupFailedUploads(ctx context.Context, olderThan time.Time) error {
	return r.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", models.OriginalModelStatusFailed, olderThan).
		Delete(&models.OriginalModel{}).Error
}

// ArchiveOldModels 归档旧模型
func (r *originalModelRepository) ArchiveOldModels(ctx context.Context, olderThan time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.OriginalModel{}).
		Where("last_accessed < ? AND status = ?", olderThan, models.OriginalModelStatusReady).
		Updates(map[string]interface{}{
			"status":        models.OriginalModelStatusReady,
			"storage_class": "cold",
		}).Error
}

// LoadConvertedModels 预加载原始模型关联的转换后模型
func (r *originalModelRepository) LoadConvertedModels(ctx context.Context, model *models.OriginalModel) error {
	return r.db.WithContext(ctx).
		Model(model).
		Preload("ConvertedModels", "status = ?", "completed").
		Association("ConvertedModels").
		Find(&model.ConvertedModels)
}
