package repository

import (
	"box-manage-service/models"

	"gorm.io/gorm"
)

type VideoFileRepository interface {
	Create(videoFile *models.VideoFile) error
	GetByID(id uint) (*models.VideoFile, error)
	List(page, pageSize int, status *models.VideoFileStatus, userID *uint) ([]*models.VideoFile, int64, error)
	Update(videoFile *models.VideoFile) error
	Delete(id uint) error
	GetByStreamID(streamID string) (*models.VideoFile, error)
	GetReadyFiles() ([]*models.VideoFile, error)
	FindByStatus(status models.VideoFileStatus) ([]*models.VideoFile, error)
}

type videoFileRepository struct {
	db *gorm.DB
}

func NewVideoFileRepository(db *gorm.DB) VideoFileRepository {
	return &videoFileRepository{db: db}
}

func (r *videoFileRepository) Create(videoFile *models.VideoFile) error {
	return r.db.Create(videoFile).Error
}

func (r *videoFileRepository) GetByID(id uint) (*models.VideoFile, error) {
	var videoFile models.VideoFile
	err := r.db.First(&videoFile, id).Error
	return &videoFile, err
}

func (r *videoFileRepository) List(page, pageSize int, status *models.VideoFileStatus, userID *uint) ([]*models.VideoFile, int64, error) {
	var videoFiles []*models.VideoFile
	var total int64

	query := r.db.Model(&models.VideoFile{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&videoFiles).Error
	return videoFiles, total, err
}

func (r *videoFileRepository) Update(videoFile *models.VideoFile) error {
	return r.db.Save(videoFile).Error
}

func (r *videoFileRepository) Delete(id uint) error {
	return r.db.Delete(&models.VideoFile{}, id).Error
}

func (r *videoFileRepository) GetByStreamID(streamID string) (*models.VideoFile, error) {
	var videoFile models.VideoFile
	err := r.db.Where("stream_id = ?", streamID).First(&videoFile).Error
	return &videoFile, err
}

func (r *videoFileRepository) GetReadyFiles() ([]*models.VideoFile, error) {
	var videoFiles []*models.VideoFile
	err := r.db.Where("status = ?", models.VideoFileStatusReady).Find(&videoFiles).Error
	return videoFiles, err
}

func (r *videoFileRepository) FindByStatus(status models.VideoFileStatus) ([]*models.VideoFile, error) {
	var videoFiles []*models.VideoFile
	err := r.db.Where("status = ?", status).Find(&videoFiles).Error
	return videoFiles, err
}
