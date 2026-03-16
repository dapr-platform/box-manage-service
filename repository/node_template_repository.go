/*
 * @module repository/node_template_repository
 * @description 节点模板Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> NodeTemplateRepository -> GORM -> 数据库
 * @rules 实现节点模板的CRUD操作、类型查询等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.2节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"

	"gorm.io/gorm"
)

// NodeTemplateRepository 节点模板Repository接口
type NodeTemplateRepository interface {
	BaseRepository[models.NodeTemplate]

	// 基础查询
	FindByType(ctx context.Context, nodeType string) ([]*models.NodeTemplate, error)
	FindByKeyName(ctx context.Context, keyName string) (*models.NodeTemplate, error)
	FindByCategory(ctx context.Context, category string) ([]*models.NodeTemplate, error)
	FindEnabled(ctx context.Context) ([]*models.NodeTemplate, error)

	// 状态管理
	Enable(ctx context.Context, id uint) error
	Disable(ctx context.Context, id uint) error

	// 搜索
	SearchTemplates(ctx context.Context, keyword string) ([]*models.NodeTemplate, error)

	// 统计
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
}

// nodeTemplateRepository 节点模板Repository实现
type nodeTemplateRepository struct {
	BaseRepository[models.NodeTemplate]
	db *gorm.DB
}

// NewNodeTemplateRepository 创建NodeTemplate Repository实例
func NewNodeTemplateRepository(db *gorm.DB) NodeTemplateRepository {
	return &nodeTemplateRepository{
		BaseRepository: newBaseRepository[models.NodeTemplate](db),
		db:             db,
	}
}

// FindByType 根据类型查找节点模板
func (r *nodeTemplateRepository) FindByType(ctx context.Context, nodeType string) ([]*models.NodeTemplate, error) {
	var templates []*models.NodeTemplate
	err := r.db.WithContext(ctx).
		Where("type_key = ?", nodeType).
		Order("sort_order ASC").
		Find(&templates).Error
	return templates, err
}

// FindByKeyName 根据key_name查找节点模板
func (r *nodeTemplateRepository) FindByKeyName(ctx context.Context, keyName string) (*models.NodeTemplate, error) {
	var template models.NodeTemplate
	err := r.db.WithContext(ctx).
		Where("key_name = ?", keyName).
		First(&template).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &template, nil
}

// FindByCategory 根据分类查找节点模板
func (r *nodeTemplateRepository) FindByCategory(ctx context.Context, category string) ([]*models.NodeTemplate, error) {
	var templates []*models.NodeTemplate
	err := r.db.WithContext(ctx).
		Where("category = ?", category).
		Order("sort_order ASC").
		Find(&templates).Error
	return templates, err
}

// FindEnabled 查找启用的节点模板
func (r *nodeTemplateRepository) FindEnabled(ctx context.Context) ([]*models.NodeTemplate, error) {
	var templates []*models.NodeTemplate
	err := r.db.WithContext(ctx).
		Where("is_enabled = ?", true).
		Order("category ASC, sort_order ASC").
		Find(&templates).Error
	return templates, err
}

// Enable 启用节点模板
func (r *nodeTemplateRepository) Enable(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.NodeTemplate{}).
		Where("id = ?", id).
		Update("is_enabled", true).Error
}

// Disable 禁用节点模板
func (r *nodeTemplateRepository) Disable(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.NodeTemplate{}).
		Where("id = ?", id).
		Update("is_enabled", false).Error
}

// SearchTemplates 搜索节点模板
func (r *nodeTemplateRepository) SearchTemplates(ctx context.Context, keyword string) ([]*models.NodeTemplate, error) {
	var templates []*models.NodeTemplate
	err := r.db.WithContext(ctx).
		Where("type_name LIKE ? OR type_key LIKE ? OR description LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%").
		Order("sort_order ASC").
		Find(&templates).Error
	return templates, err
}

// GetStatistics 获取节点模板统计信息
func (r *nodeTemplateRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总模板数
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.NodeTemplate{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 启用的模板数
	var enabled int64
	if err := r.db.WithContext(ctx).Model(&models.NodeTemplate{}).
		Where("is_enabled = ?", true).
		Count(&enabled).Error; err != nil {
		return nil, err
	}
	stats["enabled"] = enabled

	// 按分类统计
	type categoryCount struct {
		Category string
		Count    int64
	}
	var categoryCounts []categoryCount
	if err := r.db.WithContext(ctx).
		Model(&models.NodeTemplate{}).
		Select("category, count(*) as count").
		Group("category").
		Find(&categoryCounts).Error; err != nil {
		return nil, err
	}

	categoryStats := make(map[string]int64)
	for _, cc := range categoryCounts {
		categoryStats[cc.Category] = cc.Count
	}
	stats["by_category"] = categoryStats

	return stats, nil
}
