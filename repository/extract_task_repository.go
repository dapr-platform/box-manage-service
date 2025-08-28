package repository

import (
	"box-manage-service/models"

	"gorm.io/gorm"
)

// ExtractTaskRepository 定义抽帧任务数据访问接口
type ExtractTaskRepository interface {
	Create(task *models.ExtractTask) error
	GetByID(id uint) (*models.ExtractTask, error)
	GetByTaskID(taskID string) (*models.ExtractTask, error)
	Update(task *models.ExtractTask) error
	Delete(id uint) error
	List(query map[string]interface{}, offset, limit int) ([]*models.ExtractTask, int64, error)
	Count(query map[string]interface{}) (int64, error)

	// 状态相关查询
	GetByStatus(status models.ExtractTaskStatus) ([]*models.ExtractTask, error)
	FindByStatus(status models.ExtractTaskStatus) ([]*models.ExtractTask, error) // 别名方法，保持一致性
	GetActiveTasksByVideoSource(videoSourceID uint) ([]*models.ExtractTask, error)
	GetActiveTasksByVideoFile(videoFileID uint) ([]*models.ExtractTask, error)
	GetTasksByUser(userID uint, offset, limit int) ([]*models.ExtractTask, int64, error)

	// 批量操作
	BatchUpdateStatus(taskIDs []string, status models.ExtractTaskStatus) error
	DeleteByVideoSource(videoSourceID uint) error
	DeleteByVideoFile(videoFileID uint) error
}

// extractTaskRepository ExtractTaskRepository 的 GORM 实现
type extractTaskRepository struct {
	db *gorm.DB
}

// NewExtractTaskRepository 创建新的 ExtractTaskRepository 实例
func NewExtractTaskRepository(db *gorm.DB) ExtractTaskRepository {
	return &extractTaskRepository{db: db}
}

// Create 创建抽帧任务
func (r *extractTaskRepository) Create(task *models.ExtractTask) error {
	return r.db.Create(task).Error
}

// GetByID 根据ID获取抽帧任务
func (r *extractTaskRepository) GetByID(id uint) (*models.ExtractTask, error) {
	var task models.ExtractTask
	if err := r.db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// GetByTaskID 根据TaskID获取抽帧任务
func (r *extractTaskRepository) GetByTaskID(taskID string) (*models.ExtractTask, error) {
	var task models.ExtractTask
	if err := r.db.Where("task_id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// Update 更新抽帧任务
func (r *extractTaskRepository) Update(task *models.ExtractTask) error {
	return r.db.Save(task).Error
}

// Delete 删除抽帧任务
func (r *extractTaskRepository) Delete(id uint) error {
	return r.db.Delete(&models.ExtractTask{}, id).Error
}

// List 获取抽帧任务列表
func (r *extractTaskRepository) List(query map[string]interface{}, offset, limit int) ([]*models.ExtractTask, int64, error) {
	var tasks []*models.ExtractTask
	var total int64

	db := r.db.Model(&models.ExtractTask{})

	// 应用查询条件
	if len(query) > 0 {
		db = db.Where(query)
	}

	// 获取总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	if limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}

	// 按创建时间倒序
	err := db.Order("created_at DESC").Find(&tasks).Error
	return tasks, total, err
}

// Count 统计抽帧任务数量
func (r *extractTaskRepository) Count(query map[string]interface{}) (int64, error) {
	var total int64
	db := r.db.Model(&models.ExtractTask{})

	if len(query) > 0 {
		db = db.Where(query)
	}

	err := db.Count(&total).Error
	return total, err
}

// GetByStatus 根据状态获取抽帧任务
func (r *extractTaskRepository) GetByStatus(status models.ExtractTaskStatus) ([]*models.ExtractTask, error) {
	var tasks []*models.ExtractTask
	err := r.db.Where("status = ?", status).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// FindByStatus 根据状态获取抽帧任务（别名方法，保持一致性）
func (r *extractTaskRepository) FindByStatus(status models.ExtractTaskStatus) ([]*models.ExtractTask, error) {
	return r.GetByStatus(status)
}

// GetActiveTasksByVideoSource 获取指定视频源的活跃任务
func (r *extractTaskRepository) GetActiveTasksByVideoSource(videoSourceID uint) ([]*models.ExtractTask, error) {
	var tasks []*models.ExtractTask
	err := r.db.Where("video_source_id = ? AND status = ?",
		videoSourceID, models.ExtractTaskStatusExtracting).Find(&tasks).Error
	return tasks, err
}

// GetActiveTasksByVideoFile 获取指定视频文件的活跃任务
func (r *extractTaskRepository) GetActiveTasksByVideoFile(videoFileID uint) ([]*models.ExtractTask, error) {
	var tasks []*models.ExtractTask
	err := r.db.Where("video_file_id = ? AND status = ?",
		videoFileID, models.ExtractTaskStatusExtracting).Find(&tasks).Error
	return tasks, err
}

// GetTasksByUser 获取指定用户的抽帧任务
func (r *extractTaskRepository) GetTasksByUser(userID uint, offset, limit int) ([]*models.ExtractTask, int64, error) {
	var tasks []*models.ExtractTask
	var total int64

	db := r.db.Model(&models.ExtractTask{}).Where("user_id = ?", userID)

	// 获取总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	if limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}

	err := db.Order("created_at DESC").Find(&tasks).Error
	return tasks, total, err
}

// BatchUpdateStatus 批量更新任务状态
func (r *extractTaskRepository) BatchUpdateStatus(taskIDs []string, status models.ExtractTaskStatus) error {
	return r.db.Model(&models.ExtractTask{}).
		Where("task_id IN ?", taskIDs).
		Update("status", status).Error
}

// DeleteByVideoSource 删除指定视频源的所有抽帧任务
func (r *extractTaskRepository) DeleteByVideoSource(videoSourceID uint) error {
	return r.db.Where("video_source_id = ?", videoSourceID).Delete(&models.ExtractTask{}).Error
}

// DeleteByVideoFile 删除指定视频文件的所有抽帧任务
func (r *extractTaskRepository) DeleteByVideoFile(videoFileID uint) error {
	return r.db.Where("video_file_id = ?", videoFileID).Delete(&models.ExtractTask{}).Error
}
