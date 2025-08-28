package repository

import (
	"box-manage-service/models"

	"gorm.io/gorm"
)

type VideoSourceRepository interface {
	Create(videoSource *models.VideoSource) error
	GetByID(id uint) (*models.VideoSource, error)
	List(page, pageSize int, userID *uint) ([]*models.VideoSource, int64, error)
	Update(videoSource *models.VideoSource) error
	Delete(id uint) error
	GetByStreamID(streamID string) (*models.VideoSource, error)
	GetActiveStreams() ([]*models.VideoSource, error)
	UpdateStatus(id uint, status models.VideoSourceStatus, errorMsg string) error
	FindByPlayURL(playURL string) ([]*models.VideoSource, error)
	FindByURL(url string) ([]*models.VideoSource, error)
}

type videoSourceRepository struct {
	db *gorm.DB
}

func NewVideoSourceRepository(db *gorm.DB) VideoSourceRepository {
	return &videoSourceRepository{db: db}
}

func (r *videoSourceRepository) Create(videoSource *models.VideoSource) error {
	return r.db.Create(videoSource).Error
}

func (r *videoSourceRepository) GetByID(id uint) (*models.VideoSource, error) {
	var videoSource models.VideoSource
	err := r.db.First(&videoSource, id).Error
	return &videoSource, err
}

func (r *videoSourceRepository) List(page, pageSize int, userID *uint) ([]*models.VideoSource, int64, error) {
	var videoSources []*models.VideoSource
	var total int64

	query := r.db.Model(&models.VideoSource{})
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&videoSources).Error
	return videoSources, total, err
}

func (r *videoSourceRepository) Update(videoSource *models.VideoSource) error {
	return r.db.Save(videoSource).Error
}

func (r *videoSourceRepository) Delete(id uint) error {
	return r.db.Delete(&models.VideoSource{}, id).Error
}

func (r *videoSourceRepository) GetByStreamID(streamID string) (*models.VideoSource, error) {
	var videoSource models.VideoSource
	err := r.db.Where("stream_id = ?", streamID).First(&videoSource).Error
	return &videoSource, err
}

func (r *videoSourceRepository) GetActiveStreams() ([]*models.VideoSource, error) {
	var videoSources []*models.VideoSource
	err := r.db.Where("status = ? AND type = ?",
		models.VideoSourceStatusActive,
		models.VideoSourceTypeStream).Find(&videoSources).Error
	return videoSources, err
}

func (r *videoSourceRepository) UpdateStatus(id uint, status models.VideoSourceStatus, errorMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMsg != "" {
		updates["error_msg"] = errorMsg
	} else {
		updates["error_msg"] = ""
	}
	if status == models.VideoSourceStatusActive {
		updates["last_active"] = gorm.Expr("NOW()")
	}

	return r.db.Model(&models.VideoSource{}).Where("id = ?", id).Updates(updates).Error
}

func (r *videoSourceRepository) FindByPlayURL(playURL string) ([]*models.VideoSource, error) {
	var videoSources []*models.VideoSource
	err := r.db.Where("play_url = ?", playURL).Find(&videoSources).Error
	return videoSources, err
}

func (r *videoSourceRepository) FindByURL(url string) ([]*models.VideoSource, error) {
	var videoSources []*models.VideoSource
	err := r.db.Where("url = ?", url).Find(&videoSources).Error
	return videoSources, err
}
