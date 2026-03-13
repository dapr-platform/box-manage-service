/*
 * @module repository/workflow_repository
 * @description 工作流Repository实现，提供工作流相关的数据访问操作
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> WorkflowRepository -> GORM -> 数据库
 * @rules 实现工作流的CRUD操作、版本管理、状态管理等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.1节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"

	"gorm.io/gorm"
)

// WorkflowRepository 工作流Repository接口
type WorkflowRepository interface {
	BaseRepository[models.Workflow]

	// 基础查询
	FindByKeyName(ctx context.Context, keyName string) ([]*models.Workflow, error)
	FindByKeyNameAndVersion(ctx context.Context, keyName string, version int) (*models.Workflow, error)
	FindByStatus(ctx context.Context, status models.WorkflowStatus) ([]*models.Workflow, error)
	FindByCategory(ctx context.Context, category string) ([]*models.Workflow, error)
	FindEnabled(ctx context.Context) ([]*models.Workflow, error)

	// 版本管理
	GetLatestVersion(ctx context.Context, keyName string) (*models.Workflow, error)
	GetAllVersions(ctx context.Context, keyName string) ([]*models.Workflow, error)
	GetNextVersion(ctx context.Context, keyName string) (int, error)

	// 状态管理
	UpdateStatus(ctx context.Context, id uint, status models.WorkflowStatus) error
	Publish(ctx context.Context, id uint) error
	Archive(ctx context.Context, id uint) error
	Enable(ctx context.Context, id uint) error
	Disable(ctx context.Context, id uint) error

	// 搜索和筛选
	SearchWorkflows(ctx context.Context, keyword string, options *QueryOptions) ([]*models.Workflow, error)
	FindByTags(ctx context.Context, tags []string) ([]*models.Workflow, error)

	// 统计查询
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
	GetStatusDistribution(ctx context.Context) (map[models.WorkflowStatus]int64, error)

	// 关联加载
	LoadWithDefinitions(ctx context.Context, workflow *models.Workflow) error
}

// workflowRepository 工作流Repository实现
type workflowRepository struct {
	BaseRepository[models.Workflow]
	db *gorm.DB
}

// NewWorkflowRepository 创建Workflow Repository实例
func NewWorkflowRepository(db *gorm.DB) WorkflowRepository {
	return &workflowRepository{
		BaseRepository: newBaseRepository[models.Workflow](db),
		db:             db,
	}
}

// FindByKeyName 根据key_name查找工作流
func (r *workflowRepository) FindByKeyName(ctx context.Context, keyName string) ([]*models.Workflow, error) {
	var workflows []*models.Workflow
	err := r.db.WithContext(ctx).
		Where("key_name = ?", keyName).
		Order("version DESC").
		Find(&workflows).Error
	return workflows, err
}

// FindByKeyNameAndVersion 根据key_name和version查找工作流
func (r *workflowRepository) FindByKeyNameAndVersion(ctx context.Context, keyName string, version int) (*models.Workflow, error) {
	var workflow models.Workflow
	err := r.db.WithContext(ctx).
		Where("key_name = ? AND version = ?", keyName, version).
		First(&workflow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &workflow, nil
}

// FindByStatus 根据状态查找工作流
func (r *workflowRepository) FindByStatus(ctx context.Context, status models.WorkflowStatus) ([]*models.Workflow, error) {
	var workflows []*models.Workflow
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("updated_at DESC").
		Find(&workflows).Error
	return workflows, err
}

// FindByCategory 根据分类查找工作流
func (r *workflowRepository) FindByCategory(ctx context.Context, category string) ([]*models.Workflow, error) {
	var workflows []*models.Workflow
	err := r.db.WithContext(ctx).
		Where("category = ?", category).
		Order("updated_at DESC").
		Find(&workflows).Error
	return workflows, err
}

// FindEnabled 查找启用的工作流
func (r *workflowRepository) FindEnabled(ctx context.Context) ([]*models.Workflow, error) {
	var workflows []*models.Workflow
	err := r.db.WithContext(ctx).
		Where("is_enabled = ?", true).
		Order("updated_at DESC").
		Find(&workflows).Error
	return workflows, err
}

// GetLatestVersion 获取最新版本的工作流
func (r *workflowRepository) GetLatestVersion(ctx context.Context, keyName string) (*models.Workflow, error) {
	var workflow models.Workflow
	err := r.db.WithContext(ctx).
		Where("key_name = ?", keyName).
		Order("version DESC").
		First(&workflow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &workflow, nil
}

// GetAllVersions 获取所有版本的工作流
func (r *workflowRepository) GetAllVersions(ctx context.Context, keyName string) ([]*models.Workflow, error) {
	var workflows []*models.Workflow
	err := r.db.WithContext(ctx).
		Where("key_name = ?", keyName).
		Order("version DESC").
		Find(&workflows).Error
	return workflows, err
}

// GetNextVersion 获取下一个版本号
func (r *workflowRepository) GetNextVersion(ctx context.Context, keyName string) (int, error) {
	var maxVersion int
	err := r.db.WithContext(ctx).
		Model(&models.Workflow{}).
		Where("key_name = ?", keyName).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error
	if err != nil {
		return 0, err
	}
	return maxVersion + 1, nil
}

// UpdateStatus 更新工作流状态
func (r *workflowRepository) UpdateStatus(ctx context.Context, id uint, status models.WorkflowStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.Workflow{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Publish 发布工作流
func (r *workflowRepository) Publish(ctx context.Context, id uint) error {
	return r.UpdateStatus(ctx, id, models.WorkflowStatusPublished)
}

// Archive 归档工作流
func (r *workflowRepository) Archive(ctx context.Context, id uint) error {
	return r.UpdateStatus(ctx, id, models.WorkflowStatusArchived)
}

// Enable 启用工作流
func (r *workflowRepository) Enable(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.Workflow{}).
		Where("id = ?", id).
		Update("is_enabled", true).Error
}

// Disable 禁用工作流
func (r *workflowRepository) Disable(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&models.Workflow{}).
		Where("id = ?", id).
		Update("is_enabled", false).Error
}

// SearchWorkflows 搜索工作流
func (r *workflowRepository) SearchWorkflows(ctx context.Context, keyword string, options *QueryOptions) ([]*models.Workflow, error) {
	var workflows []*models.Workflow

	query := r.db.WithContext(ctx).
		Where("name LIKE ? OR key_name LIKE ? OR description LIKE ? OR tags LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")

	query = ApplyQueryOptions[models.Workflow](query, options)

	err := query.Find(&workflows).Error
	return workflows, err
}

// FindByTags 根据标签查找工作流
func (r *workflowRepository) FindByTags(ctx context.Context, tags []string) ([]*models.Workflow, error) {
	var workflows []*models.Workflow

	query := r.db.WithContext(ctx)
	for _, tag := range tags {
		query = query.Where("tags LIKE ?", "%"+tag+"%")
	}

	err := query.Order("updated_at DESC").Find(&workflows).Error
	return workflows, err
}

// GetStatistics 获取工作流统计信息
func (r *workflowRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总工作流数
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.Workflow{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 已发布工作流数
	var published int64
	if err := r.db.WithContext(ctx).Model(&models.Workflow{}).
		Where("status = ?", models.WorkflowStatusPublished).
		Count(&published).Error; err != nil {
		return nil, err
	}
	stats["published"] = published

	// 草稿工作流数
	var draft int64
	if err := r.db.WithContext(ctx).Model(&models.Workflow{}).
		Where("status = ?", models.WorkflowStatusDraft).
		Count(&draft).Error; err != nil {
		return nil, err
	}
	stats["draft"] = draft

	// 已归档工作流数
	var archived int64
	if err := r.db.WithContext(ctx).Model(&models.Workflow{}).
		Where("status = ?", models.WorkflowStatusArchived).
		Count(&archived).Error; err != nil {
		return nil, err
	}
	stats["archived"] = archived

	// 启用的工作流数
	var enabled int64
	if err := r.db.WithContext(ctx).Model(&models.Workflow{}).
		Where("is_enabled = ?", true).
		Count(&enabled).Error; err != nil {
		return nil, err
	}
	stats["enabled"] = enabled

	return stats, nil
}

// GetStatusDistribution 获取工作流状态分布
func (r *workflowRepository) GetStatusDistribution(ctx context.Context) (map[models.WorkflowStatus]int64, error) {
	distribution := make(map[models.WorkflowStatus]int64)

	type statusCount struct {
		Status models.WorkflowStatus
		Count  int64
	}

	var results []statusCount
	err := r.db.WithContext(ctx).
		Model(&models.Workflow{}).
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

// LoadWithDefinitions 加载工作流及其关联的定义
func (r *workflowRepository) LoadWithDefinitions(ctx context.Context, workflow *models.Workflow) error {
	return r.db.WithContext(ctx).
		Preload("NodeDefinitions").
		Preload("LineDefinitions").
		Preload("VariableDefinitions").
		First(workflow, workflow.ID).Error
}
