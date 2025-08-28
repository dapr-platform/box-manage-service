/*
 * @module repository/base_repository
 * @description 基础Repository实现，提供通用CRUD操作
 * @architecture 数据访问层
 * @documentReference DESIGN-000.md
 * @stateFlow Repository接口 -> 基础实现 -> GORM操作 -> 数据库
 * @rules 遵循GORM最佳实践，实现事务管理、软删除、分页等功能
 * @dependencies gorm.io/gorm
 * @refs context7 GORM最佳实践
 */

package repository

import (
	"context"
	"fmt"
	"math"

	"gorm.io/gorm"
)

// baseRepository 基础Repository实现
type baseRepository[T any] struct {
	db *gorm.DB
}

// newBaseRepository 创建基础Repository实例
func newBaseRepository[T any](db *gorm.DB) BaseRepository[T] {
	return &baseRepository[T]{db: db}
}

// Create 创建实体
func (r *baseRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// GetByID 根据ID获取实体
func (r *baseRepository[T]) GetByID(ctx context.Context, id uint) (*T, error) {
	var entity T
	err := r.db.WithContext(ctx).First(&entity, id).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// Update 更新实体
func (r *baseRepository[T]) Update(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

// Delete 硬删除实体
func (r *baseRepository[T]) Delete(ctx context.Context, id uint) error {
	var entity T
	return r.db.WithContext(ctx).Unscoped().Delete(&entity, id).Error
}

// SoftDelete 软删除实体
func (r *baseRepository[T]) SoftDelete(ctx context.Context, id uint) error {
	var entity T
	return r.db.WithContext(ctx).Delete(&entity, id).Error
}

// CreateBatch 批量创建实体
func (r *baseRepository[T]) CreateBatch(ctx context.Context, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(entities, 100).Error
}

// UpdateBatch 批量更新实体
func (r *baseRepository[T]) UpdateBatch(ctx context.Context, entities []*T) error {
	if len(entities) == 0 {
		return nil
	}

	// 在事务中执行批量更新
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, entity := range entities {
			if err := tx.Save(entity).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteBatch 批量删除实体
func (r *baseRepository[T]) DeleteBatch(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	var entity T
	return r.db.WithContext(ctx).Delete(&entity, ids).Error
}

// Find 根据条件查找实体
func (r *baseRepository[T]) Find(ctx context.Context, conditions map[string]interface{}) ([]*T, error) {
	var entities []*T

	query := r.db.WithContext(ctx)
	for key, value := range conditions {
		query = query.Where(key, value)
	}

	err := query.Find(&entities).Error
	return entities, err
}

// FindWithPagination 分页查找实体
func (r *baseRepository[T]) FindWithPagination(ctx context.Context, conditions map[string]interface{}, page, pageSize int) ([]*T, int64, error) {
	var entities []*T
	var total int64

	// 构建基础查询
	baseQuery := r.db.WithContext(ctx)
	for key, value := range conditions {
		baseQuery = baseQuery.Where(key, value)
	}

	// 获取总数
	var countEntity T
	countQuery := baseQuery.Model(&countEntity)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 获取分页数据
	err := baseQuery.Offset(offset).Limit(pageSize).Find(&entities).Error
	return entities, total, err
}

// Count 计算满足条件的实体数量
func (r *baseRepository[T]) Count(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	var count int64
	var entity T

	query := r.db.WithContext(ctx).Model(&entity)
	for key, value := range conditions {
		query = query.Where(key, value)
	}

	err := query.Count(&count).Error
	return count, err
}

// Exists 检查是否存在满足条件的实体
func (r *baseRepository[T]) Exists(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	count, err := r.Count(ctx, conditions)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// repositoryManager Repository管理器实现
type repositoryManager struct {
	db                 *gorm.DB
	boxRepo            BoxRepository
	taskRepo           TaskRepository
	deploymentTaskRepo DeploymentRepository
	upgradeRepo        UpgradeRepository
	upgradePackageRepo UpgradePackageRepository
	boxHeartbeatRepo   BoxHeartbeatRepository
	// 原始模型管理相关 repository
	originalModelRepo      OriginalModelRepository
	uploadSessionRepo      UploadSessionRepository
	modelTagRepo           ModelTagRepository
	modelDeploymentRepo    ModelDeploymentRepository
	modelBoxDeploymentRepo ModelBoxDeploymentRepository
	// 视频管理相关 repository - REQ-009
	videoSourceRepo  VideoSourceRepository
	videoFileRepo    VideoFileRepository
	extractFrameRepo ExtractFrameRepository
	extractTaskRepo  ExtractTaskRepository
	recordTaskRepo   RecordTaskRepository
}

// NewRepositoryManager 创建Repository管理器
func NewRepositoryManager(db *gorm.DB) RepositoryManager {
	return &repositoryManager{
		db:                 db,
		boxRepo:            NewBoxRepository(db),
		taskRepo:           NewTaskRepository(db),
		deploymentTaskRepo: NewDeploymentRepository(db),
		upgradeRepo:        NewUpgradeRepository(db),
		upgradePackageRepo: NewUpgradePackageRepository(db),
		boxHeartbeatRepo:   NewBoxHeartbeatRepository(db),
		// 原始模型管理相关 repository
		originalModelRepo:      NewOriginalModelRepository(db),
		uploadSessionRepo:      NewUploadSessionRepository(db),
		modelTagRepo:           NewModelTagRepository(db),
		modelDeploymentRepo:    NewModelDeploymentRepository(db),
		modelBoxDeploymentRepo: NewModelBoxDeploymentRepository(db),
		// 视频管理相关 repository - REQ-009
		videoSourceRepo:  NewVideoSourceRepository(db),
		videoFileRepo:    NewVideoFileRepository(db),
		extractFrameRepo: NewExtractFrameRepository(db),
		extractTaskRepo:  NewExtractTaskRepository(db),
		recordTaskRepo:   NewRecordTaskRepository(db),
	}
}

// Box 获取Box Repository
func (rm *repositoryManager) Box() BoxRepository {
	return rm.boxRepo
}

// Task 获取Task Repository
func (rm *repositoryManager) Task() TaskRepository {
	return rm.taskRepo
}

// DeploymentTask 获取DeploymentTask Repository
func (rm *repositoryManager) DeploymentTask() DeploymentRepository {
	return rm.deploymentTaskRepo
}

// Upgrade 获取Upgrade Repository
func (rm *repositoryManager) Upgrade() UpgradeRepository {
	return rm.upgradeRepo
}

// UpgradePackage 获取UpgradePackage Repository
func (rm *repositoryManager) UpgradePackage() UpgradePackageRepository {
	return rm.upgradePackageRepo
}

// BoxHeartbeat 获取BoxHeartbeat Repository
func (rm *repositoryManager) BoxHeartbeat() BoxHeartbeatRepository {
	return rm.boxHeartbeatRepo
}

// OriginalModel 获取OriginalModel Repository
func (rm *repositoryManager) OriginalModel() OriginalModelRepository {
	return rm.originalModelRepo
}

// UploadSession 获取UploadSession Repository
func (rm *repositoryManager) UploadSession() UploadSessionRepository {
	return rm.uploadSessionRepo
}

// ModelTag 获取ModelTag Repository
func (rm *repositoryManager) ModelTag() ModelTagRepository {
	return rm.modelTagRepo
}

// ModelDeployment 获取ModelDeployment Repository
func (rm *repositoryManager) ModelDeployment() ModelDeploymentRepository {
	return rm.modelDeploymentRepo
}

// ModelBoxDeployment 获取ModelBoxDeployment Repository
func (rm *repositoryManager) ModelBoxDeployment() ModelBoxDeploymentRepository {
	return rm.modelBoxDeploymentRepo
}

// VideoSource 获取VideoSource Repository - REQ-009
func (rm *repositoryManager) VideoSource() VideoSourceRepository {
	return rm.videoSourceRepo
}

// VideoFile 获取VideoFile Repository - REQ-009
func (rm *repositoryManager) VideoFile() VideoFileRepository {
	return rm.videoFileRepo
}

// ExtractFrame 获取ExtractFrame Repository - REQ-009
func (rm *repositoryManager) ExtractFrame() ExtractFrameRepository {
	return rm.extractFrameRepo
}

// ExtractTask 获取ExtractTask Repository - REQ-009
func (rm *repositoryManager) ExtractTask() ExtractTaskRepository {
	return rm.extractTaskRepo
}

// RecordTask 获取RecordTask Repository - REQ-009
func (rm *repositoryManager) RecordTask() RecordTaskRepository {
	return rm.recordTaskRepo
}

// Transaction 执行事务
func (rm *repositoryManager) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return rm.db.WithContext(ctx).Transaction(fn)
}

// DB 获取数据库连接
func (rm *repositoryManager) DB() *gorm.DB {
	return rm.db
}

// BuildPaginatedResult 构建分页结果
func BuildPaginatedResult[T any](data []*T, total int64, page, pageSize int) *PaginatedResult[T] {
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	return &PaginatedResult[T]{
		Data:       data,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

// ApplyQueryOptions 应用查询选项
func ApplyQueryOptions[T any](query *gorm.DB, options *QueryOptions) *gorm.DB {
	if options == nil {
		return query
	}

	// 应用预加载
	for _, preload := range options.Preload {
		query = query.Preload(preload)
	}

	// 应用排序
	if options.Sort != nil && options.Sort.Field != "" {
		order := options.Sort.Field
		if options.Sort.Order == "desc" {
			order += " DESC"
		} else {
			order += " ASC"
		}
		query = query.Order(order)
	}

	return query
}

// ValidateID 验证ID是否有效
func ValidateID(id uint) error {
	if id == 0 {
		return fmt.Errorf("invalid ID: %d", id)
	}
	return nil
}
