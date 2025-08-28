/*
 * @module repository/converted_model_repository
 * @description 转换后模型Repository实现，提供转换后模型的数据访问操作
 * @architecture 数据访问层
 * @documentReference REQ-004: 转换后模型管理
 * @stateFlow Service层 -> ConvertedModelRepository -> GORM -> 数据库
 * @rules 实现转换后模型的CRUD操作、搜索、统计等功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs DESIGN-007.md, context7 GORM最佳实践
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ConvertedModelRepository 转换后模型Repository接口
type ConvertedModelRepository interface {
	// 基本CRUD操作
	Create(ctx context.Context, model *models.ConvertedModel) error
	GetByID(ctx context.Context, id uint) (*models.ConvertedModel, error)
	Update(ctx context.Context, model *models.ConvertedModel) error
	Delete(ctx context.Context, id uint) error

	// 查询操作
	GetList(ctx context.Context, req *GetConvertedModelListRequest) (*GetConvertedModelListResponse, error)
	GetByOriginalModelID(ctx context.Context, originalModelID uint) ([]*models.ConvertedModel, error)
	GetByConversionTaskID(ctx context.Context, taskID string) (*models.ConvertedModel, error)
	GetByName(ctx context.Context, name string) (*models.ConvertedModel, error)
	GetByTargetChip(ctx context.Context, chip string) ([]*models.ConvertedModel, error)

	// 部署相关
	GetActiveModels(ctx context.Context) ([]*models.ConvertedModel, error)
	GetDeployableModels(ctx context.Context, chip string) ([]*models.ConvertedModel, error)
	GetModelsByStatus(ctx context.Context, status models.ConvertedModelStatus) ([]*models.ConvertedModel, error)

	// 搜索和筛选
	SearchModels(ctx context.Context, req *SearchConvertedModelRequest) (*SearchConvertedModelResponse, error)
	GetModelsByTags(ctx context.Context, tags []string) ([]*models.ConvertedModel, error)
	GetModelsByUserID(ctx context.Context, userID uint) ([]*models.ConvertedModel, error)

	// 统计和分析
	GetStatistics(ctx context.Context, userID *uint) (*ConvertedModelStatistics, error)
	GetPerformanceRanking(ctx context.Context, chip string, limit int) ([]*models.ConvertedModel, error)
	GetUsageStatistics(ctx context.Context, days int) (*UsageStatistics, error)

	// 批量操作
	BulkUpdateStatus(ctx context.Context, ids []uint, status models.ConvertedModelStatus) error
	BulkDelete(ctx context.Context, ids []uint) error
	BulkUpdateActive(ctx context.Context, ids []uint, isActive bool) error

	// 维护操作
	CleanupExpiredModels(ctx context.Context) (int64, error)
	CleanupInactiveModels(ctx context.Context, days int) (int64, error)
	UpdateAccessTime(ctx context.Context, id uint) error
	IncrementDownloadCount(ctx context.Context, id uint) error

	// 验证和检查
	IsNameExists(ctx context.Context, name string, excludeID ...uint) (bool, error)
	CheckDuplicateConversion(ctx context.Context, originalModelID uint, targetChip string, precision string) (*models.ConvertedModel, error)
}

// convertedModelRepository 转换后模型Repository实现
type convertedModelRepository struct {
	db *gorm.DB
}

// NewConvertedModelRepository 创建转换后模型Repository实例
func NewConvertedModelRepository(db *gorm.DB) ConvertedModelRepository {
	return &convertedModelRepository{
		db: db,
	}
}

// Create 创建转换后模型
func (r *convertedModelRepository) Create(ctx context.Context, model *models.ConvertedModel) error {
	return r.db.WithContext(ctx).Create(model).Error
}

// GetByID 根据ID获取转换后模型
func (r *convertedModelRepository) GetByID(ctx context.Context, id uint) (*models.ConvertedModel, error) {
	var model models.ConvertedModel
	err := r.db.WithContext(ctx).
		Preload("OriginalModel").
		First(&model, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

// Update 更新转换后模型
func (r *convertedModelRepository) Update(ctx context.Context, model *models.ConvertedModel) error {
	return r.db.WithContext(ctx).Save(model).Error
}

// Delete 删除转换后模型（软删除）
func (r *convertedModelRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ConvertedModel{}, id).Error
}

// GetList 获取转换后模型列表
func (r *convertedModelRepository) GetList(ctx context.Context, req *GetConvertedModelListRequest) (*GetConvertedModelListResponse, error) {
	var models []*models.ConvertedModel
	var total int64

	query := r.db.WithContext(ctx).Table("converted_models")

	// 应用筛选条件
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.TargetChip != "" {
		query = query.Where("target_chip = ?", req.TargetChip)
	}
	if req.UserID != nil {
		query = query.Where("user_id = ?", *req.UserID)
	}
	if req.OriginalModelID != nil {
		query = query.Where("original_model_id = ?", *req.OriginalModelID)
	}
	if len(req.Tags) > 0 {
		for _, tag := range req.Tags {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		}
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 应用排序
	if req.Sort != nil {
		orderStr := fmt.Sprintf("%s %s", req.Sort.Field, req.Sort.Order)
		query = query.Order(orderStr)
	} else {
		query = query.Order("created_at DESC")
	}

	// 应用分页
	if req.Pagination != nil {
		offset := (req.Pagination.Page - 1) * req.Pagination.PageSize
		query = query.Offset(offset).Limit(req.Pagination.PageSize)
	}

	// 预加载关联数据
	query = query.Preload("OriginalModel")

	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	page := 1
	pageSize := 20
	if req.Pagination != nil {
		page = req.Pagination.Page
		pageSize = req.Pagination.PageSize
	}

	return &GetConvertedModelListResponse{
		Models:   models,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetByOriginalModelID 根据原始模型ID获取转换后模型列表
func (r *convertedModelRepository) GetByOriginalModelID(ctx context.Context, originalModelID uint) ([]*models.ConvertedModel, error) {
	var models []*models.ConvertedModel
	err := r.db.WithContext(ctx).
		Where("original_model_id = ?", originalModelID).
		Order("created_at DESC").
		Find(&models).Error
	return models, err
}

// GetByConversionTaskID 根据转换任务ID获取转换后模型
func (r *convertedModelRepository) GetByConversionTaskID(ctx context.Context, taskID string) (*models.ConvertedModel, error) {
	var model models.ConvertedModel
	err := r.db.WithContext(ctx).
		Where("conversion_task_id = ?", taskID).
		First(&model).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

// GetByName 根据名称获取转换后模型
func (r *convertedModelRepository) GetByName(ctx context.Context, name string) (*models.ConvertedModel, error) {
	var model models.ConvertedModel
	err := r.db.WithContext(ctx).
		Where("name = ?", name).
		First(&model).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

// GetByTargetChip 根据目标芯片获取转换后模型列表
func (r *convertedModelRepository) GetByTargetChip(ctx context.Context, chip string) ([]*models.ConvertedModel, error) {
	var models []*models.ConvertedModel
	err := r.db.WithContext(ctx).
		Where("status = ?", "completed").
		Order("created_at DESC").
		Find(&models).Error
	return models, err
}

// GetActiveModels 获取激活的转换后模型（简化为已完成状态的模型）
func (r *convertedModelRepository) GetActiveModels(ctx context.Context) ([]*models.ConvertedModel, error) {
	var models []*models.ConvertedModel
	err := r.db.WithContext(ctx).
		Where("status = ?", "completed").
		Order("created_at DESC").
		Find(&models).Error
	return models, err
}

// GetDeployableModels 获取可部署的转换后模型（简化为已完成状态的模型）
func (r *convertedModelRepository) GetDeployableModels(ctx context.Context, chip string) ([]*models.ConvertedModel, error) {
	var models []*models.ConvertedModel
	query := r.db.WithContext(ctx).
		Where("status = ?", "completed")

	if chip != "" {
		query = query.Where("target_chip = ?", chip)
	}

	err := query.Order("created_at DESC").Find(&models).Error
	return models, err
}

// GetModelsByStatus 根据状态获取转换后模型
func (r *convertedModelRepository) GetModelsByStatus(ctx context.Context, status models.ConvertedModelStatus) ([]*models.ConvertedModel, error) {
	var models []*models.ConvertedModel
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at DESC").
		Find(&models).Error
	return models, err
}

// SearchModels 搜索转换后模型
func (r *convertedModelRepository) SearchModels(ctx context.Context, req *SearchConvertedModelRequest) (*SearchConvertedModelResponse, error) {
	var models []*models.ConvertedModel
	var total int64

	query := r.db.WithContext(ctx).Table("converted_models")

	// 关键词搜索
	if req.Keyword != "" {
		searchPattern := "%" + req.Keyword + "%"
		query = query.Where("name LIKE ? OR display_name LIKE ? OR description LIKE ? OR tags LIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern)
	}

	// 其他筛选条件
	if req.TargetChip != "" {
		query = query.Where("target_chip = ?", req.TargetChip)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.UserID != nil {
		query = query.Where("user_id = ?", *req.UserID)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页和排序
	if req.Pagination != nil {
		offset := (req.Pagination.Page - 1) * req.Pagination.PageSize
		query = query.Offset(offset).Limit(req.Pagination.PageSize)
	}

	query = query.Order("created_at DESC").Preload("OriginalModel")

	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	page := 1
	pageSize := 20
	if req.Pagination != nil {
		page = req.Pagination.Page
		pageSize = req.Pagination.PageSize
	}

	return &SearchConvertedModelResponse{
		Models:   models,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Keyword:  req.Keyword,
	}, nil
}

// GetModelsByTags 根据标签获取转换后模型
func (r *convertedModelRepository) GetModelsByTags(ctx context.Context, tags []string) ([]*models.ConvertedModel, error) {
	var models []*models.ConvertedModel
	query := r.db.WithContext(ctx)

	for _, tag := range tags {
		query = query.Where("tags LIKE ?", "%"+tag+"%")
	}

	err := query.Order("created_at DESC").Find(&models).Error
	return models, err
}

// GetModelsByUserID 根据用户ID获取转换后模型
func (r *convertedModelRepository) GetModelsByUserID(ctx context.Context, userID uint) ([]*models.ConvertedModel, error) {
	var models []*models.ConvertedModel
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&models).Error
	return models, err
}

// GetStatistics 获取转换后模型统计信息
func (r *convertedModelRepository) GetStatistics(ctx context.Context, userID *uint) (*ConvertedModelStatistics, error) {
	stats := &ConvertedModelStatistics{}

	query := r.db.WithContext(ctx)
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	// 总数统计
	if err := query.Count(&stats.TotalModels).Error; err != nil {
		return nil, err
	}

	// 按状态统计
	statusStats := make(map[string]int64)
	rows, err := query.Select("status, count(*) as count").Group("status").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		statusStats[status] = count
	}
	stats.StatusStats = statusStats

	// 按芯片统计
	chipStats := make(map[string]int64)
	rows, err = query.Select("target_chip, count(*) as count").Group("target_chip").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var chip string
		var count int64
		if err := rows.Scan(&chip, &count); err != nil {
			continue
		}
		chipStats[chip] = count
	}
	stats.ChipStats = chipStats

	// 已完成模型数（原激活模型统计已简化）
	if err := query.Where("status = ?", "completed").Count(&stats.ActiveModels).Error; err != nil {
		return nil, err
	}

	// 已部署模型数
	if err := query.Where("deployment_count > ?", 0).Count(&stats.DeployedModels).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// GetPerformanceRanking 获取性能排名
func (r *convertedModelRepository) GetPerformanceRanking(ctx context.Context, chip string, limit int) ([]*models.ConvertedModel, error) {
	var models []*models.ConvertedModel
	query := r.db.WithContext(ctx).
		Where("status = ?", "completed")

	if chip != "" {
		query = query.Where("target_chip = ?", chip)
	}

	err := query.
		Order("benchmark_score DESC, accuracy_score DESC").
		Limit(limit).
		Find(&models).Error

	return models, err
}

// GetUsageStatistics 获取使用统计
func (r *convertedModelRepository) GetUsageStatistics(ctx context.Context, days int) (*UsageStatistics, error) {
	stats := &UsageStatistics{}

	since := time.Now().AddDate(0, 0, -days)

	// 总访问次数
	err := r.db.WithContext(ctx).
		Model(&models.ConvertedModel{}).
		Select("COALESCE(SUM(access_count), 0)").
		Where("created_at >= ?", since).
		Scan(&stats.TotalAccess).Error
	if err != nil {
		return nil, err
	}

	// 总下载次数
	err = r.db.WithContext(ctx).
		Model(&models.ConvertedModel{}).
		Select("COALESCE(SUM(download_count), 0)").
		Where("created_at >= ?", since).
		Scan(&stats.TotalDownloads).Error
	if err != nil {
		return nil, err
	}

	// 总部署次数
	err = r.db.WithContext(ctx).
		Model(&models.ConvertedModel{}).
		Select("COALESCE(SUM(deployment_count), 0)").
		Where("created_at >= ?", since).
		Scan(&stats.TotalDeployments).Error
	if err != nil {
		return nil, err
	}

	// 新增模型数
	err = r.db.WithContext(ctx).
		Model(&models.ConvertedModel{}).
		Where("created_at >= ?", since).
		Count(&stats.NewModels).Error
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// BulkUpdateStatus 批量更新状态
func (r *convertedModelRepository) BulkUpdateStatus(ctx context.Context, ids []uint, status models.ConvertedModelStatus) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&models.ConvertedModel{}).
		Where("id IN ?", ids).
		Update("status", status).Error
}

// BulkDelete 批量删除
func (r *convertedModelRepository) BulkDelete(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Delete(&models.ConvertedModel{}, ids).Error
}

// BulkUpdateActive 批量更新激活状态（字段已移除，方法保留但无操作）
func (r *convertedModelRepository) BulkUpdateActive(ctx context.Context, ids []uint, isActive bool) error {
	// is_active字段已从模型中移除，此方法不再执行任何操作
	return nil
}

// CleanupExpiredModels 清理过期模型
func (r *convertedModelRepository) CleanupExpiredModels(ctx context.Context) (int64, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Delete(&models.ConvertedModel{}, "expires_at IS NOT NULL AND expires_at < ?", now)
	return result.RowsAffected, result.Error
}

// CleanupInactiveModels 清理长期不活跃的模型（简化逻辑）
func (r *convertedModelRepository) CleanupInactiveModels(ctx context.Context, days int) (int64, error) {
	threshold := time.Now().AddDate(0, 0, -days)
	result := r.db.WithContext(ctx).
		Model(&models.ConvertedModel{}).
		Where("last_accessed_at IS NULL OR last_accessed_at < ?", threshold).
		Update("storage_class", "cold")
	return result.RowsAffected, result.Error
}

// UpdateAccessTime 更新访问时间
func (r *convertedModelRepository) UpdateAccessTime(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.ConvertedModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"access_count":     gorm.Expr("access_count + 1"),
			"last_accessed_at": now,
		}).Error
}

// IncrementDownloadCount 增加下载计数
func (r *convertedModelRepository) IncrementDownloadCount(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.ConvertedModel{}).
		Where("id = ?", id).
		Update("download_count", gorm.Expr("download_count + 1")).Error
}

// IsNameExists 检查名称是否存在
func (r *convertedModelRepository) IsNameExists(ctx context.Context, name string, excludeID ...uint) (bool, error) {
	query := r.db.WithContext(ctx).Where("name = ?", name)

	if len(excludeID) > 0 && excludeID[0] > 0 {
		query = query.Where("id != ?", excludeID[0])
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

// CheckDuplicateConversion 检查重复转换
func (r *convertedModelRepository) CheckDuplicateConversion(ctx context.Context, originalModelID uint, targetChip string, precision string) (*models.ConvertedModel, error) {
	var model models.ConvertedModel
	err := r.db.WithContext(ctx).
		Where("original_model_id = ? AND target_chip = ? AND precision = ? AND status = ?",
			originalModelID, targetChip, precision, "completed").
		First(&model).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

// 请求和响应结构体

// GetConvertedModelListRequest 获取转换后模型列表请求
type GetConvertedModelListRequest struct {
	Status          string             `json:"status,omitempty"`
	TargetChip      string             `json:"target_chip,omitempty"`
	UserID          *uint              `json:"user_id,omitempty"`
	OriginalModelID *uint              `json:"original_model_id,omitempty"`
	Tags            []string           `json:"tags,omitempty"`
	Pagination      *PaginationOptions `json:"pagination,omitempty"`
	Sort            *SortOptions       `json:"sort,omitempty"`
}

// GetConvertedModelListResponse 获取转换后模型列表响应
type GetConvertedModelListResponse struct {
	Models   []*models.ConvertedModel `json:"models"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

// SearchConvertedModelRequest 搜索转换后模型请求
type SearchConvertedModelRequest struct {
	Keyword    string             `json:"keyword"`
	TargetChip string             `json:"target_chip,omitempty"`
	Status     string             `json:"status,omitempty"`
	UserID     *uint              `json:"user_id,omitempty"`
	Pagination *PaginationOptions `json:"pagination,omitempty"`
}

// SearchConvertedModelResponse 搜索转换后模型响应
type SearchConvertedModelResponse struct {
	Models   []*models.ConvertedModel `json:"models"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Keyword  string                   `json:"keyword"`
}

// ConvertedModelStatistics 转换后模型统计信息
type ConvertedModelStatistics struct {
	TotalModels    int64            `json:"total_models"`
	ActiveModels   int64            `json:"active_models"`
	DeployedModels int64            `json:"deployed_models"`
	StatusStats    map[string]int64 `json:"status_stats"`
	ChipStats      map[string]int64 `json:"chip_stats"`
}

// UsageStatistics 使用统计信息
type UsageStatistics struct {
	TotalAccess      int64 `json:"total_access"`
	TotalDownloads   int64 `json:"total_downloads"`
	TotalDeployments int64 `json:"total_deployments"`
	NewModels        int64 `json:"new_models"`
}
