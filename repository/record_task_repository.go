package repository

import (
	"box-manage-service/models"

	"gorm.io/gorm"
)

type RecordTaskRepository interface {
	Create(task *models.RecordTask) error
	GetByID(id uint) (*models.RecordTask, error)
	List(page, pageSize int, status *models.RecordTaskStatus, videoSourceID *uint) ([]*models.RecordTask, int64, error)
	Update(task *models.RecordTask) error
	Delete(id uint) error
	GetByTaskID(taskID string) (*models.RecordTask, error)
	GetActiveRecords() ([]*models.RecordTask, error)
	Count(status *models.RecordTaskStatus, userID *uint) (int64, error)
	FindByStatus(status models.RecordTaskStatus) ([]*models.RecordTask, error)
}

type recordTaskRepository struct {
	db *gorm.DB
}

func NewRecordTaskRepository(db *gorm.DB) RecordTaskRepository {
	return &recordTaskRepository{db: db}
}

func (r *recordTaskRepository) Create(task *models.RecordTask) error {
	return r.db.Create(task).Error
}

func (r *recordTaskRepository) GetByID(id uint) (*models.RecordTask, error) {
	var task models.RecordTask
	err := r.db.First(&task, id).Error
	return &task, err
}

func (r *recordTaskRepository) List(page, pageSize int, status *models.RecordTaskStatus, videoSourceID *uint) ([]*models.RecordTask, int64, error) {
	var tasks []*models.RecordTask
	var total int64

	query := r.db.Model(&models.RecordTask{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if videoSourceID != nil {
		query = query.Where("video_source_id = ?", *videoSourceID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tasks).Error
	return tasks, total, err
}

func (r *recordTaskRepository) Update(task *models.RecordTask) error {
	return r.db.Save(task).Error
}

func (r *recordTaskRepository) Delete(id uint) error {
	return r.db.Delete(&models.RecordTask{}, id).Error
}

func (r *recordTaskRepository) GetByTaskID(taskID string) (*models.RecordTask, error) {
	var task models.RecordTask
	err := r.db.Where("task_id = ?", taskID).First(&task).Error
	return &task, err
}

func (r *recordTaskRepository) GetActiveRecords() ([]*models.RecordTask, error) {
	var tasks []*models.RecordTask
	err := r.db.Where("status = ?", models.RecordTaskStatusRecording).Find(&tasks).Error
	return tasks, err
}

func (r *recordTaskRepository) Count(status *models.RecordTaskStatus, userID *uint) (int64, error) {
	var count int64
	query := r.db.Model(&models.RecordTask{})

	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	err := query.Count(&count).Error
	return count, err
}

func (r *recordTaskRepository) FindByStatus(status models.RecordTaskStatus) ([]*models.RecordTask, error) {
	var tasks []*models.RecordTask
	err := r.db.Where("status = ?", status).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}
