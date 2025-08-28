/*
 * @module repository/model_deployment_repository
 * @description 模型部署任务数据访问层
 * @architecture 数据访问层
 * @documentReference REQ-004: 模型部署功能
 * @stateFlow Service -> Repository -> Database
 * @rules 实现模型部署任务的数据库操作
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

// ModelDeploymentRepository 模型部署数据访问接口
type ModelDeploymentRepository interface {
	// 任务CRUD操作
	CreateTask(ctx context.Context, task *models.ModelDeploymentTask) error
	GetTaskByID(ctx context.Context, id uint) (*models.ModelDeploymentTask, error)
	GetTaskByTaskID(ctx context.Context, taskID string) (*models.ModelDeploymentTask, error)
	UpdateTask(ctx context.Context, task *models.ModelDeploymentTask) error
	DeleteTask(ctx context.Context, id uint) error

	// 任务查询
	GetTasksByUserID(ctx context.Context, userID uint) ([]*models.ModelDeploymentTask, error)
	GetTasksByStatus(ctx context.Context, status models.ModelDeploymentStatus) ([]*models.ModelDeploymentTask, error)
	GetRecentTasks(ctx context.Context, limit int) ([]*models.ModelDeploymentTask, error)

	// 部署项CRUD操作
	CreateItem(ctx context.Context, item *models.ModelDeploymentItem) error
	GetItemByID(ctx context.Context, id uint) (*models.ModelDeploymentItem, error)
	UpdateItem(ctx context.Context, item *models.ModelDeploymentItem) error
	GetItemsByTaskID(ctx context.Context, taskID string) ([]*models.ModelDeploymentItem, error)

	// 批量操作
	CreateItems(ctx context.Context, items []*models.ModelDeploymentItem) error
	UpdateTaskWithItems(ctx context.Context, task *models.ModelDeploymentTask, items []*models.ModelDeploymentItem) error

	// 清理操作
	CleanupOldTasks(ctx context.Context, olderThan time.Time) (int64, error)
}

// modelDeploymentRepository 模型部署数据访问实现
type modelDeploymentRepository struct {
	db *gorm.DB
}

// NewModelDeploymentRepository 创建模型部署数据访问实例
func NewModelDeploymentRepository(db *gorm.DB) ModelDeploymentRepository {
	return &modelDeploymentRepository{db: db}
}

// CreateTask 创建部署任务
func (r *modelDeploymentRepository) CreateTask(ctx context.Context, task *models.ModelDeploymentTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// GetTaskByID 根据ID获取部署任务
func (r *modelDeploymentRepository) GetTaskByID(ctx context.Context, id uint) (*models.ModelDeploymentTask, error) {
	var task models.ModelDeploymentTask
	err := r.db.WithContext(ctx).
		Preload("ConvertedModels").
		Preload("Boxes").
		Preload("DeploymentItems").
		First(&task, id).Error

	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTaskByTaskID 根据任务ID获取部署任务
func (r *modelDeploymentRepository) GetTaskByTaskID(ctx context.Context, taskID string) (*models.ModelDeploymentTask, error) {
	var task models.ModelDeploymentTask
	err := r.db.WithContext(ctx).
		Preload("ConvertedModels").
		Preload("Boxes").
		Preload("DeploymentItems").
		Where("task_id = ?", taskID).
		First(&task).Error

	if err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTask 更新部署任务
func (r *modelDeploymentRepository) UpdateTask(ctx context.Context, task *models.ModelDeploymentTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

// DeleteTask 删除部署任务
func (r *modelDeploymentRepository) DeleteTask(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先删除部署项
		if err := tx.Where("task_id = (SELECT task_id FROM model_deployment_tasks WHERE id = ?)", id).
			Delete(&models.ModelDeploymentItem{}).Error; err != nil {
			return err
		}

		// 删除任务
		return tx.Delete(&models.ModelDeploymentTask{}, id).Error
	})
}

// GetTasksByUserID 根据用户ID获取部署任务
func (r *modelDeploymentRepository) GetTasksByUserID(ctx context.Context, userID uint) ([]*models.ModelDeploymentTask, error) {
	var tasks []*models.ModelDeploymentTask
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&tasks).Error

	return tasks, err
}

// GetTasksByStatus 根据状态获取部署任务
func (r *modelDeploymentRepository) GetTasksByStatus(ctx context.Context, status models.ModelDeploymentStatus) ([]*models.ModelDeploymentTask, error) {
	var tasks []*models.ModelDeploymentTask
	err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at ASC").
		Find(&tasks).Error

	return tasks, err
}

// GetRecentTasks 获取最近的部署任务
func (r *modelDeploymentRepository) GetRecentTasks(ctx context.Context, limit int) ([]*models.ModelDeploymentTask, error) {
	var tasks []*models.ModelDeploymentTask
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&tasks).Error

	return tasks, err
}

// CreateItem 创建部署项
func (r *modelDeploymentRepository) CreateItem(ctx context.Context, item *models.ModelDeploymentItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

// GetItemByID 根据ID获取部署项
func (r *modelDeploymentRepository) GetItemByID(ctx context.Context, id uint) (*models.ModelDeploymentItem, error) {
	var item models.ModelDeploymentItem
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		First(&item, id).Error

	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateItem 更新部署项
func (r *modelDeploymentRepository) UpdateItem(ctx context.Context, item *models.ModelDeploymentItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

// GetItemsByTaskID 根据任务ID获取部署项
func (r *modelDeploymentRepository) GetItemsByTaskID(ctx context.Context, taskID string) ([]*models.ModelDeploymentItem, error) {
	var items []*models.ModelDeploymentItem
	err := r.db.WithContext(ctx).
		Preload("ConvertedModel").
		Preload("Box").
		Where("task_id = ?", taskID).
		Order("created_at ASC").
		Find(&items).Error

	return items, err
}

// CreateItems 批量创建部署项
func (r *modelDeploymentRepository) CreateItems(ctx context.Context, items []*models.ModelDeploymentItem) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(items, 100).Error
}

// UpdateTaskWithItems 更新任务和部署项
func (r *modelDeploymentRepository) UpdateTaskWithItems(ctx context.Context, task *models.ModelDeploymentTask, items []*models.ModelDeploymentItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 更新任务
		if err := tx.Save(task).Error; err != nil {
			return err
		}

		// 批量更新部署项
		for _, item := range items {
			if err := tx.Save(item).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// CleanupOldTasks 清理旧任务
func (r *modelDeploymentRepository) CleanupOldTasks(ctx context.Context, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ? AND status IN (?)", olderThan, []string{"completed", "failed", "cancelled"}).
		Delete(&models.ModelDeploymentTask{})

	return result.RowsAffected, result.Error
}
