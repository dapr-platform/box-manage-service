/*
 * @module repository/model_box_deployment_repository
 * @description 模型-盒子部署关联关系数据访问层
 * @architecture 数据访问层
 * @documentReference REQ-004: 模型部署功能
 * @stateFlow Service -> Repository -> Database
 * @rules 实现模型-盒子部署关联关系的数据库操作
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs DESIGN-007.md
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// ModelBoxDeploymentRepository 模型-盒子部署关联数据访问接口
type ModelBoxDeploymentRepository interface {
	// 基础CRUD操作
	Create(ctx context.Context, deployment *models.ModelBoxDeployment) error
	GetByID(ctx context.Context, id uint) (*models.ModelBoxDeployment, error)
	Update(ctx context.Context, deployment *models.ModelBoxDeployment) error
	Delete(ctx context.Context, id uint) error

	// 查询操作
	GetByModelAndBox(ctx context.Context, convertedModelID, boxID uint) (*models.ModelBoxDeployment, error)
	GetByModelKey(ctx context.Context, modelKey string) ([]*models.ModelBoxDeployment, error)
	GetByConvertedModelID(ctx context.Context, convertedModelID uint) ([]*models.ModelBoxDeployment, error)
	GetByBoxID(ctx context.Context, boxID uint) ([]*models.ModelBoxDeployment, error)
	GetActiveDeployments(ctx context.Context) ([]*models.ModelBoxDeployment, error)
	GetByStatus(ctx context.Context, status models.ModelBoxDeploymentStatus) ([]*models.ModelBoxDeployment, error)

	// 批量操作
	CreateBatch(ctx context.Context, deployments []*models.ModelBoxDeployment) error
	UpdateStatusBatch(ctx context.Context, ids []uint, status models.ModelBoxDeploymentStatus) error

	// 业务方法
	ExistsActiveDeployment(ctx context.Context, convertedModelID, boxID uint) (bool, error)
	GetDeploymentsByTaskID(ctx context.Context, taskID string) ([]*models.ModelBoxDeployment, error)
	MarkBoxModelsInactive(ctx context.Context, boxID uint, reason string) error
	CleanupRemovedDeployments(ctx context.Context, olderThan time.Time) (int64, error)

	// 统计操作
	CountByModel(ctx context.Context, convertedModelID uint) (int64, error)
	CountByBox(ctx context.Context, boxID uint) (int64, error)
	CountActiveDeployments(ctx context.Context) (int64, error)
}

// modelBoxDeploymentRepository 模型-盒子部署关联数据访问实现
type modelBoxDeploymentRepository struct {
	db *gorm.DB
}

// NewModelBoxDeploymentRepository 创建模型-盒子部署关联数据访问实例
func NewModelBoxDeploymentRepository(db *gorm.DB) ModelBoxDeploymentRepository {
	return &modelBoxDeploymentRepository{db: db}
}

// Create 创建部署关联
func (r *modelBoxDeploymentRepository) Create(ctx context.Context, deployment *models.ModelBoxDeployment) error {
	return r.db.WithContext(ctx).Create(deployment).Error
}

// GetByID 根据ID获取部署关联
func (r *modelBoxDeploymentRepository) GetByID(ctx context.Context, id uint) (*models.ModelBoxDeployment, error) {
	var deployment models.ModelBoxDeployment
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		First(&deployment, id).Error

	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

// Update 更新部署关联
func (r *modelBoxDeploymentRepository) Update(ctx context.Context, deployment *models.ModelBoxDeployment) error {
	return r.db.WithContext(ctx).Save(deployment).Error
}

// Delete 删除部署关联
func (r *modelBoxDeploymentRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.ModelBoxDeployment{}, id).Error
}

// GetByModelAndBox 根据模型和盒子获取部署关联
func (r *modelBoxDeploymentRepository) GetByModelAndBox(ctx context.Context, convertedModelID, boxID uint) (*models.ModelBoxDeployment, error) {
	var deployment models.ModelBoxDeployment
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		Where("converted_model_id = ? AND box_id = ? AND status != ?",
			convertedModelID, boxID, models.ModelBoxDeploymentStatusRemoved).
		First(&deployment).Error

	if err != nil {
		return nil, err
	}
	return &deployment, nil
}

// GetByModelKey 根据模型Key获取所有部署关联
func (r *modelBoxDeploymentRepository) GetByModelKey(ctx context.Context, modelKey string) ([]*models.ModelBoxDeployment, error) {
	var deployments []*models.ModelBoxDeployment
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		Where("model_key = ? AND status != ?", modelKey, models.ModelBoxDeploymentStatusRemoved).
		Order("deployed_at DESC").
		Find(&deployments).Error

	return deployments, err
}

// GetByConvertedModelID 根据转换后模型ID获取所有部署关联
func (r *modelBoxDeploymentRepository) GetByConvertedModelID(ctx context.Context, convertedModelID uint) ([]*models.ModelBoxDeployment, error) {
	var deployments []*models.ModelBoxDeployment
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		Where("converted_model_id = ? AND status != ?", convertedModelID, models.ModelBoxDeploymentStatusRemoved).
		Order("deployed_at DESC").
		Find(&deployments).Error

	return deployments, err
}

// GetByBoxID 根据盒子ID获取所有部署关联
func (r *modelBoxDeploymentRepository) GetByBoxID(ctx context.Context, boxID uint) ([]*models.ModelBoxDeployment, error) {
	var deployments []*models.ModelBoxDeployment
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		Where("box_id = ? AND status != ?", boxID, models.ModelBoxDeploymentStatusRemoved).
		Order("deployed_at DESC").
		Find(&deployments).Error

	return deployments, err
}

// GetActiveDeployments 获取所有活跃的部署关联
func (r *modelBoxDeploymentRepository) GetActiveDeployments(ctx context.Context) ([]*models.ModelBoxDeployment, error) {
	var deployments []*models.ModelBoxDeployment
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		Where("status = ?", models.ModelBoxDeploymentStatusActive).
		Order("deployed_at DESC").
		Find(&deployments).Error

	return deployments, err
}

// GetByStatus 根据状态获取部署关联
func (r *modelBoxDeploymentRepository) GetByStatus(ctx context.Context, status models.ModelBoxDeploymentStatus) ([]*models.ModelBoxDeployment, error) {
	var deployments []*models.ModelBoxDeployment
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		Where("status = ?", status).
		Order("deployed_at DESC").
		Find(&deployments).Error

	return deployments, err
}

// CreateBatch 批量创建部署关联
func (r *modelBoxDeploymentRepository) CreateBatch(ctx context.Context, deployments []*models.ModelBoxDeployment) error {
	if len(deployments) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(deployments, 100).Error
}

// UpdateStatusBatch 批量更新状态
func (r *modelBoxDeploymentRepository) UpdateStatusBatch(ctx context.Context, ids []uint, status models.ModelBoxDeploymentStatus) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&models.ModelBoxDeployment{}).
		Where("id IN ?", ids).
		Update("status", status).Error
}

// ExistsActiveDeployment 检查是否存在活跃的部署关联
func (r *modelBoxDeploymentRepository) ExistsActiveDeployment(ctx context.Context, convertedModelID, boxID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ModelBoxDeployment{}).
		Where("converted_model_id = ? AND box_id = ? AND status = ?",
			convertedModelID, boxID, models.ModelBoxDeploymentStatusActive).
		Count(&count).Error

	return count > 0, err
}

// GetDeploymentsByTaskID 根据任务ID获取部署关联
func (r *modelBoxDeploymentRepository) GetDeploymentsByTaskID(ctx context.Context, taskID string) ([]*models.ModelBoxDeployment, error) {
	var deployments []*models.ModelBoxDeployment
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		Where("task_id = ?", taskID).
		Order("deployed_at DESC").
		Find(&deployments).Error

	return deployments, err
}

// MarkBoxModelsInactive 将指定盒子的所有模型标记为不活跃
func (r *modelBoxDeploymentRepository) MarkBoxModelsInactive(ctx context.Context, boxID uint, reason string) error {
	return r.db.WithContext(ctx).
		Model(&models.ModelBoxDeployment{}).
		Where("box_id = ? AND status = ?", boxID, models.ModelBoxDeploymentStatusActive).
		Updates(map[string]interface{}{
			"status":           models.ModelBoxDeploymentStatusInactive,
			"verification_msg": reason,
			"is_verified":      false,
		}).Error
}

// CleanupRemovedDeployments 清理已移除的部署关联
func (r *modelBoxDeploymentRepository) CleanupRemovedDeployments(ctx context.Context, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", models.ModelBoxDeploymentStatusRemoved, olderThan).
		Delete(&models.ModelBoxDeployment{})

	return result.RowsAffected, result.Error
}

// CountByModel 统计指定模型的部署数量
func (r *modelBoxDeploymentRepository) CountByModel(ctx context.Context, convertedModelID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ModelBoxDeployment{}).
		Where("converted_model_id = ? AND status != ?", convertedModelID, models.ModelBoxDeploymentStatusRemoved).
		Count(&count).Error

	return count, err
}

// CountByBox 统计指定盒子的部署数量
func (r *modelBoxDeploymentRepository) CountByBox(ctx context.Context, boxID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ModelBoxDeployment{}).
		Where("box_id = ? AND status != ?", boxID, models.ModelBoxDeploymentStatusRemoved).
		Count(&count).Error

	return count, err
}

// CountActiveDeployments 统计活跃部署数量
func (r *modelBoxDeploymentRepository) CountActiveDeployments(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ModelBoxDeployment{}).
		Where("status = ?", models.ModelBoxDeploymentStatusActive).
		Count(&count).Error

	return count, err
}
