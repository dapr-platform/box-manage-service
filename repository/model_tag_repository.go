/*
 * @module repository/model_tag_repository
 * @description 模型标签Repository实现，提供标签管理相关的数据访问操作
 * @architecture 数据访问层
 * @documentReference REQ-002: 原始模型管理
 * @stateFlow Service层 -> ModelTagRepository -> GORM -> 数据库
 * @rules 实现标签的CRUD操作、使用统计、清理等功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs REQ-002.md
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"

	"gorm.io/gorm"
)

// modelTagRepository 模型标签Repository实现
type modelTagRepository struct {
	BaseRepository[models.ModelTag]
	db *gorm.DB
}

// NewModelTagRepository 创建模型标签Repository实例
func NewModelTagRepository(db *gorm.DB) ModelTagRepository {
	return &modelTagRepository{
		BaseRepository: newBaseRepository[models.ModelTag](db),
		db:             db,
	}
}

// FindByName 根据名称查找标签
func (r *modelTagRepository) FindByName(ctx context.Context, name string) (*models.ModelTag, error) {
	var tag models.ModelTag
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&tag).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tag, nil
}

// FindPopularTags 查找热门标签
func (r *modelTagRepository) FindPopularTags(ctx context.Context, limit int) ([]*models.ModelTag, error) {
	var tags []*models.ModelTag
	err := r.db.WithContext(ctx).
		Order("usage DESC").
		Limit(limit).
		Find(&tags).Error
	return tags, err
}

// SearchTags 搜索标签
func (r *modelTagRepository) SearchTags(ctx context.Context, keyword string) ([]*models.ModelTag, error) {
	var tags []*models.ModelTag
	err := r.db.WithContext(ctx).
		Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%").
		Order("usage DESC").
		Find(&tags).Error
	return tags, err
}

// IncrementUsage 增加标签使用次数
func (r *modelTagRepository) IncrementUsage(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.ModelTag{}).
		Where("id = ?", id).
		UpdateColumn("usage", gorm.Expr("usage + 1")).Error
}

// DecrementUsage 减少标签使用次数
func (r *modelTagRepository) DecrementUsage(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.ModelTag{}).
		Where("id = ? AND usage > 0", id).
		UpdateColumn("usage", gorm.Expr("usage - 1")).Error
}

// UpdateUsageCount 更新标签使用次数
func (r *modelTagRepository) UpdateUsageCount(ctx context.Context, id uint, count int) error {
	return r.db.WithContext(ctx).
		Model(&models.ModelTag{}).
		Where("id = ?", id).
		Update("usage", count).Error
}

// GetOrCreateTag 获取或创建标签
func (r *modelTagRepository) GetOrCreateTag(ctx context.Context, name string) (*models.ModelTag, error) {
	// 首先尝试查找现有标签
	tag, err := r.FindByName(ctx, name)
	if err != nil {
		return nil, err
	}

	// 如果标签存在，返回现有标签
	if tag != nil {
		return tag, nil
	}

	// 如果标签不存在，创建新标签
	newTag := &models.ModelTag{
		Name:        name,
		Description: "",
		Color:       "#1890ff", // 默认蓝色
		Usage:       0,
	}

	err = r.Create(ctx, newTag)
	if err != nil {
		return nil, err
	}

	return newTag, nil
}

// CleanupUnusedTags 清理未使用的标签
func (r *modelTagRepository) CleanupUnusedTags(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("usage = 0").
		Delete(&models.ModelTag{}).Error
}
