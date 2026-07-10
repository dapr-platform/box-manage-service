package repository

import (
	"box-manage-service/models"
	"context"

	"gorm.io/gorm"
)

// TaskScheduleHistoryRepository 任务调度历史仓储接口
type TaskScheduleHistoryRepository interface {
	BaseRepository[models.TaskScheduleHistory]
	FindByTaskID(ctx context.Context, taskID uint, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error)
	FindByBoxID(ctx context.Context, boxID uint, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error)
	FindAll(ctx context.Context, page, pageSize int) ([]*models.TaskScheduleHistory, int64, error)
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
