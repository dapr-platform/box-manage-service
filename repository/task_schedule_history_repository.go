package repository

import (
	"box-manage-service/models"
	"context"
	"time"

	"gorm.io/gorm"
)

// TaskScheduleHistoryFilter 调度历史查询过滤条件（各字段为空表示不限制）
type TaskScheduleHistoryFilter struct {
	TaskID    *uint      // 按任务过滤
	BoxID     *uint      // 按盒子过滤
	Trigger   string     // 触发类型：manual / auto
	Keyword   string     // 关键词：匹配任务名或盒子名
	StartTime *time.Time // 起始时间（created_at >=）
	EndTime   *time.Time // 结束时间（created_at <=）
}

// TaskScheduleHistoryRepository 任务调度历史仓储接口
type TaskScheduleHistoryRepository interface {
	BaseRepository[models.TaskScheduleHistory]
	FindByTaskID(ctx context.Context, taskID uint, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error)
	FindByBoxID(ctx context.Context, boxID uint, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error)
	FindAll(ctx context.Context, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error)
	// FindWithFilter 按多条件组合过滤 + 分页查询
	FindWithFilter(ctx context.Context, filter TaskScheduleHistoryFilter, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error)
}

type taskScheduleHistoryRepository struct {
	BaseRepository[models.TaskScheduleHistory]
	db *gorm.DB
}

func NewTaskScheduleHistoryRepository(db *gorm.DB) TaskScheduleHistoryRepository {
	return &taskScheduleHistoryRepository{
		BaseRepository: newBaseRepository[models.TaskScheduleHistory](db),
		db:             db,
	}
}

func (r *taskScheduleHistoryRepository) FindByTaskID(ctx context.Context, taskID uint, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error) {
	return r.find(ctx, "task_id = ?", taskID, page, pageSize)
}

func (r *taskScheduleHistoryRepository) FindByBoxID(ctx context.Context, boxID uint, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error) {
	return r.find(ctx, "box_id = ?", boxID, page, pageSize)
}

func (r *taskScheduleHistoryRepository) FindAll(ctx context.Context, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error) {
	return r.find(ctx, "", nil, page, pageSize)
}

// FindWithFilter 支持按任务/盒子/触发类型/关键词/时间范围组合过滤
func (r *taskScheduleHistoryRepository) FindWithFilter(ctx context.Context, filter TaskScheduleHistoryFilter, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error) {
	var list []*models.TaskScheduleHistory
	var total int64

	query := r.db.WithContext(ctx).Model(&models.TaskScheduleHistory{})

	if filter.TaskID != nil {
		query = query.Where("task_id = ?", *filter.TaskID)
	}
	if filter.BoxID != nil {
		query = query.Where("box_id = ?", *filter.BoxID)
	}
	if filter.Trigger != "" {
		query = query.Where("trigger = ?", filter.Trigger)
	}
	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		query = query.Where("task_name LIKE ? OR box_name LIKE ?", like, like)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", *filter.EndTime)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *taskScheduleHistoryRepository) find(ctx context.Context, where string, arg interface{}, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error) {
	var list []*models.TaskScheduleHistory
	var total int64

	query := r.db.WithContext(ctx).Model(&models.TaskScheduleHistory{})
	if where != "" {
		query = query.Where(where, arg)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error
	return list, total, err
}
