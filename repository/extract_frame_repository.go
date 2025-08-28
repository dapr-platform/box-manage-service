package repository

import (
	"box-manage-service/models"

	"gorm.io/gorm"
)

type ExtractFrameRepository interface {
	Create(frame *models.ExtractFrame) error
	GetByID(id uint) (*models.ExtractFrame, error)
	List(page, pageSize int, taskID *string) ([]*models.ExtractFrame, int64, error)
	Delete(id uint) error
	GetByTaskID(taskID string) ([]*models.ExtractFrame, error)
}

type extractFrameRepository struct {
	db *gorm.DB
}

func NewExtractFrameRepository(db *gorm.DB) ExtractFrameRepository {
	return &extractFrameRepository{db: db}
}

func (r *extractFrameRepository) Create(frame *models.ExtractFrame) error {
	return r.db.Create(frame).Error
}

func (r *extractFrameRepository) GetByID(id uint) (*models.ExtractFrame, error) {
	var frame models.ExtractFrame
	err := r.db.First(&frame, id).Error
	return &frame, err
}

func (r *extractFrameRepository) List(page, pageSize int, taskID *string) ([]*models.ExtractFrame, int64, error) {
	var frames []*models.ExtractFrame
	var total int64

	query := r.db.Model(&models.ExtractFrame{})
	if taskID != nil {
		query = query.Where("task_id = ?", *taskID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&frames).Error
	return frames, total, err
}

func (r *extractFrameRepository) Delete(id uint) error {
	return r.db.Delete(&models.ExtractFrame{}, id).Error
}

func (r *extractFrameRepository) GetByTaskID(taskID string) ([]*models.ExtractFrame, error) {
	var frames []*models.ExtractFrame
	err := r.db.Where("task_id = ?", taskID).Order("frame_index ASC").Find(&frames).Error
	return frames, err
}

