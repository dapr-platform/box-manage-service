/*
 * @module repository/schedule_policy_repository
 * @description 调度策略Repository实现
 * @architecture 数据访问层
 * @documentReference REQ-005: 任务管理功能
 * @rules 实现调度策略的CRUD操作
 * @dependencies gorm.io/gorm
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"fmt"

	"gorm.io/gorm"
)

// schedulePolicyRepository 调度策略Repository实现
type schedulePolicyRepository struct {
	db *gorm.DB
}

// NewSchedulePolicyRepository 创建调度策略Repository实例
func NewSchedulePolicyRepository(db *gorm.DB) SchedulePolicyRepository {
	return &schedulePolicyRepository{db: db}
}

// Create 创建调度策略
func (r *schedulePolicyRepository) Create(ctx context.Context, policy *models.SchedulePolicy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

// GetByID 根据ID获取调度策略
func (r *schedulePolicyRepository) GetByID(ctx context.Context, id uint) (*models.SchedulePolicy, error) {
	var policy models.SchedulePolicy
	if err := r.db.WithContext(ctx).First(&policy, id).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

// Update 更新调度策略
func (r *schedulePolicyRepository) Update(ctx context.Context, policy *models.SchedulePolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

// Delete 删除调度策略
func (r *schedulePolicyRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.SchedulePolicy{}, id).Error
}

// SoftDelete 软删除调度策略
func (r *schedulePolicyRepository) SoftDelete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.SchedulePolicy{}, id).Error
}

// CreateBatch 批量创建调度策略
func (r *schedulePolicyRepository) CreateBatch(ctx context.Context, policies []*models.SchedulePolicy) error {
	return r.db.WithContext(ctx).Create(&policies).Error
}

// UpdateBatch 批量更新调度策略
func (r *schedulePolicyRepository) UpdateBatch(ctx context.Context, policies []*models.SchedulePolicy) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, policy := range policies {
			if err := tx.Save(policy).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteBatch 批量删除调度策略
func (r *schedulePolicyRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	return r.db.WithContext(ctx).Delete(&models.SchedulePolicy{}, ids).Error
}

// Find 根据条件查询调度策略
func (r *schedulePolicyRepository) Find(ctx context.Context, conditions map[string]interface{}) ([]*models.SchedulePolicy, error) {
	var policies []*models.SchedulePolicy
	query := r.db.WithContext(ctx)

	if conditions != nil {
		query = query.Where(conditions)
	}

	if err := query.Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

// FindWithPagination 分页查询调度策略
func (r *schedulePolicyRepository) FindWithPagination(ctx context.Context, conditions map[string]interface{}, page, pageSize int) ([]*models.SchedulePolicy, int64, error) {
	var policies []*models.SchedulePolicy
	var total int64

	query := r.db.WithContext(ctx).Model(&models.SchedulePolicy{})

	if conditions != nil {
		query = query.Where(conditions)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("priority DESC, created_at DESC").Find(&policies).Error; err != nil {
		return nil, 0, err
	}

	return policies, total, nil
}

// Count 统计数量
func (r *schedulePolicyRepository) Count(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.SchedulePolicy{})

	if conditions != nil {
		query = query.Where(conditions)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Exists 检查是否存在
func (r *schedulePolicyRepository) Exists(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	count, err := r.Count(ctx, conditions)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByName 根据名称查找调度策略
func (r *schedulePolicyRepository) FindByName(ctx context.Context, name string) (*models.SchedulePolicy, error) {
	var policy models.SchedulePolicy
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

// FindByType 根据类型查找调度策略
func (r *schedulePolicyRepository) FindByType(ctx context.Context, policyType models.SchedulePolicyType) ([]*models.SchedulePolicy, error) {
	var policies []*models.SchedulePolicy
	if err := r.db.WithContext(ctx).Where("policy_type = ?", policyType).Order("priority DESC").Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

// FindEnabled 查找所有启用的调度策略
func (r *schedulePolicyRepository) FindEnabled(ctx context.Context) ([]*models.SchedulePolicy, error) {
	var policies []*models.SchedulePolicy
	if err := r.db.WithContext(ctx).Where("is_enabled = ?", true).Order("priority DESC").Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

// Enable 启用调度策略
func (r *schedulePolicyRepository) Enable(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.SchedulePolicy{}).Where("id = ?", id).Update("is_enabled", true).Error
}

// Disable 禁用调度策略
func (r *schedulePolicyRepository) Disable(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.SchedulePolicy{}).Where("id = ?", id).Update("is_enabled", false).Error
}

// UpdatePriority 更新策略优先级
func (r *schedulePolicyRepository) UpdatePriority(ctx context.Context, id uint, priority int) error {
	return r.db.WithContext(ctx).Model(&models.SchedulePolicy{}).Where("id = ?", id).Update("priority", priority).Error
}

// FindEnabledByPriority 按优先级排序获取启用的策略
func (r *schedulePolicyRepository) FindEnabledByPriority(ctx context.Context) ([]*models.SchedulePolicy, error) {
	var policies []*models.SchedulePolicy
	if err := r.db.WithContext(ctx).
		Where("is_enabled = ?", true).
		Order("priority DESC, created_at ASC").
		Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

// GetDefaultPolicy 获取默认策略（优先级最高的启用策略）
func (r *schedulePolicyRepository) GetDefaultPolicy(ctx context.Context) (*models.SchedulePolicy, error) {
	var policy models.SchedulePolicy
	if err := r.db.WithContext(ctx).
		Where("is_enabled = ?", true).
		Order("priority DESC").
		First(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

// GetStatistics 获取调度策略统计信息
func (r *schedulePolicyRepository) GetStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.SchedulePolicy{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("统计总数失败: %w", err)
	}
	stats["total"] = total

	// 启用数量
	var enabled int64
	if err := r.db.WithContext(ctx).Model(&models.SchedulePolicy{}).Where("is_enabled = ?", true).Count(&enabled).Error; err != nil {
		return nil, fmt.Errorf("统计启用数量失败: %w", err)
	}
	stats["enabled"] = enabled
	stats["disabled"] = total - enabled

	// 按类型统计
	type TypeCount struct {
		PolicyType models.SchedulePolicyType
		Count      int64
	}
	var typeCounts []TypeCount
	if err := r.db.WithContext(ctx).
		Model(&models.SchedulePolicy{}).
		Select("policy_type, count(*) as count").
		Group("policy_type").
		Find(&typeCounts).Error; err != nil {
		return nil, fmt.Errorf("统计类型分布失败: %w", err)
	}

	typeDistribution := make(map[string]int64)
	for _, tc := range typeCounts {
		typeDistribution[string(tc.PolicyType)] = tc.Count
	}
	stats["type_distribution"] = typeDistribution

	return stats, nil
}

