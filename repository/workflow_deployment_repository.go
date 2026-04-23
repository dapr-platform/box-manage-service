/*
 * @module repository/workflow_deployment_repository
 * @description 工作流部署Repository实现
 * @architecture 数据访问层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Service层 -> WorkflowDeploymentRepository -> GORM -> 数据库
 * @rules 实现工作流部署的CRUD操作、状态管理等功能
 * @dependencies gorm.io/gorm
 * @refs 业务编排引擎需求文档.md 4.1.11节
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// WorkflowDeploymentRepository 工作流部署Repository接口
type WorkflowDeploymentRepository interface {
	BaseRepository[models.WorkflowDeployment]

	// 基础查询
	FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowDeployment, error)
	FindByBoxID(ctx context.Context, boxID uint) ([]*models.WorkflowDeployment, error)
	FindByWorkflowIDAndBoxID(ctx context.Context, workflowID uint, boxID uint) ([]*models.WorkflowDeployment, error)
	FindByStatus(ctx context.Context, status models.DeploymentStatus) ([]*models.WorkflowDeployment, error)
	GetLatestDeployment(ctx context.Context, workflowID uint, boxID uint) (*models.WorkflowDeployment, error)

	// 分页查询
	FindWithFilters(ctx context.Context, workflowID *uint, boxID *uint, page, pageSize int) ([]*models.WorkflowDeployment, int64, error)

	// 状态管理
	UpdateStatus(ctx context.Context, id uint, status models.DeploymentStatus) error
	MarkAsDeployed(ctx context.Context, id uint) error
	MarkAsFailed(ctx context.Context, id uint, errorMsg string) error
	MarkAsRolledBack(ctx context.Context, id uint) error

	// 统计
	GetStatistics(ctx context.Context) (map[string]interface{}, error)
}

// workflowDeploymentRepository 工作流部署Repository实现
type workflowDeploymentRepository struct {
	BaseRepository[models.WorkflowDeployment]
	db *gorm.DB
}

// NewWorkflowDeploymentRepository 创建WorkflowDeployment Repository实例
func NewWorkflowDeploymentRepository(db *gorm.DB) WorkflowDeploymentRepository {
	return &workflowDeploymentRepository{
		BaseRepository: newBaseRepository[models.WorkflowDeployment](db),
		db:             db,
	}
}

// FindByWorkflowID 根据工作流ID查找部署记录
func (r *workflowDeploymentRepository) FindByWorkflowID(ctx context.Context, workflowID uint) ([]*models.WorkflowDeployment, error) {
	var deployments []*models.WorkflowDeployment
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("created_at DESC").
		Find(&deployments).Error
	return deployments, err
}

// FindByBoxID 根据盒子ID查找部署记录
func (r *workflowDeploymentRepository) FindByBoxID(ctx context.Context, boxID uint) ([]*models.WorkflowDeployment, error) {
	var deployments []*models.WorkflowDeployment
	err := r.db.WithContext(ctx).
		Where("box_id = ?", boxID).
		Order("created_at DESC").
		Find(&deployments).Error
	return deployments, err
}

// FindByWorkflowIDAndBoxID 根据工作流ID和盒子ID查找部署记录
func (r *workflowDeploymentRepository) FindByWorkflowIDAndBoxID(ctx context.Context, workflowID uint, boxID uint) ([]*models.WorkflowDeployment, error) {
	var deployments []*models.WorkflowDeployment
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND box_id = ?", workflowID, boxID).
		Order("created_at DESC").
		Find(&deployments).Error
	return deployments, err
}

// FindByStatus 根据状态查找部署记录
func (r *workflowDeploymentRepository) FindByStatus(ctx context.Context, status models.DeploymentStatus) ([]*models.WorkflowDeployment, error) {
	var deployments []*models.WorkflowDeployment
	err := r.db.WithContext(ctx).
		Where("deployment_status = ?", status).
		Order("created_at DESC").
		Find(&deployments).Error
	return deployments, err
}

// GetLatestDeployment 获取最新的部署记录
func (r *workflowDeploymentRepository) GetLatestDeployment(ctx context.Context, workflowID uint, boxID uint) (*models.WorkflowDeployment, error) {
	var deployment models.WorkflowDeployment
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND box_id = ?", workflowID, boxID).
		Order("created_at DESC").
		First(&deployment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &deployment, nil
}

// UpdateStatus 更新部署状态
func (r *workflowDeploymentRepository) UpdateStatus(ctx context.Context, id uint, status models.DeploymentStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowDeployment{}).
		Where("id = ?", id).
		Update("deployment_status", status).Error
}

// MarkAsDeployed 标记为已部署
func (r *workflowDeploymentRepository) MarkAsDeployed(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WorkflowDeployment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"deployment_status": models.DeploymentStatusDeployed,
			"deployed_at":       now,
		}).Error
}

// MarkAsFailed 标记为失败
func (r *workflowDeploymentRepository) MarkAsFailed(ctx context.Context, id uint, errorMsg string) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkflowDeployment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"deployment_status": models.DeploymentStatusFailed,
			"error_message":     errorMsg,
		}).Error
}

// MarkAsRolledBack 标记为已回滚
func (r *workflowDeploymentRepository) MarkAsRolledBack(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WorkflowDeployment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"deployment_status": models.DeploymentStatusRolledBack,
			"rolled_back_at":    now,
		}).Error
}

// GetStatistics 获取部署统计信息
func (r *workflowDeploymentRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总部署数
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowDeployment{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// 已部署数
	var deployed int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowDeployment{}).
		Where("deployment_status = ?", models.DeploymentStatusDeployed).
		Count(&deployed).Error; err != nil {
		return nil, err
	}
	stats["deployed"] = deployed

	// 失败数
	var failed int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowDeployment{}).
		Where("deployment_status = ?", models.DeploymentStatusFailed).
		Count(&failed).Error; err != nil {
		return nil, err
	}
	stats["failed"] = failed

	// 回滚数
	var rolledBack int64
	if err := r.db.WithContext(ctx).Model(&models.WorkflowDeployment{}).
		Where("deployment_status = ?", models.DeploymentStatusRolledBack).
		Count(&rolledBack).Error; err != nil {
		return nil, err
	}
	stats["rolled_back"] = rolledBack

	return stats, nil
}

// FindWithFilters 根据筛选条件分页查询部署记录
func (r *workflowDeploymentRepository) FindWithFilters(ctx context.Context, workflowID *uint, boxID *uint, page, pageSize int) ([]*models.WorkflowDeployment, int64, error) {
	var deployments []*models.WorkflowDeployment
	var total int64

	query := r.db.WithContext(ctx).Model(&models.WorkflowDeployment{})

	// 应用筛选条件
	if workflowID != nil {
		query = query.Where("workflow_id = ?", *workflowID)
	}
	if boxID != nil {
		query = query.Where("box_id = ?", *boxID)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 应用分页
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&deployments).Error; err != nil {
		return nil, 0, err
	}

	return deployments, total, nil
}
