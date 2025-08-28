/*
 * @module repository/upload_session_repository
 * @description 上传会话Repository实现，提供文件上传会话管理
 * @architecture 数据访问层
 * @documentReference REQ-002: 原始模型管理
 * @stateFlow Service层 -> UploadSessionRepository -> GORM -> 数据库
 * @rules 实现上传会话的CRUD操作、状态管理、过期清理等功能
 * @dependencies gorm.io/gorm, box-manage-service/models
 * @refs REQ-002.md, DESIGN-004.md
 */

package repository

import (
	"box-manage-service/models"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// uploadSessionRepository 上传会话Repository实现
type uploadSessionRepository struct {
	BaseRepository[models.UploadSession]
	db *gorm.DB
}

// NewUploadSessionRepository 创建上传会话Repository实例
func NewUploadSessionRepository(db *gorm.DB) UploadSessionRepository {
	return &uploadSessionRepository{
		BaseRepository: newBaseRepository[models.UploadSession](db),
		db:             db,
	}
}

// FindBySessionID 根据会话ID查找上传会话
func (r *uploadSessionRepository) FindBySessionID(ctx context.Context, sessionID string) (*models.UploadSession, error) {
	var session models.UploadSession
	err := r.db.WithContext(ctx).Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// FindByUserID 根据用户ID查找上传会话
func (r *uploadSessionRepository) FindByUserID(ctx context.Context, userID uint) ([]*models.UploadSession, error) {
	var sessions []*models.UploadSession
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// FindActiveByUserID 根据用户ID查找活跃的上传会话
func (r *uploadSessionRepository) FindActiveByUserID(ctx context.Context, userID uint) ([]*models.UploadSession, error) {
	var sessions []*models.UploadSession
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status IN (?)", userID, []models.UploadStatus{
			models.UploadStatusInitialized,
			models.UploadStatusUploading,
		}).
		Order("created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// FindExpiredSessions 查找过期的上传会话
func (r *uploadSessionRepository) FindExpiredSessions(ctx context.Context) ([]*models.UploadSession, error) {
	var sessions []*models.UploadSession
	err := r.db.WithContext(ctx).
		Where("expires_at < ? AND status NOT IN (?)", time.Now(), []models.UploadStatus{
			models.UploadStatusCompleted,
			models.UploadStatusFailed,
			models.UploadStatusCancelled,
		}).
		Find(&sessions).Error
	return sessions, err
}

// UpdateStatus 更新上传会话状态
func (r *uploadSessionRepository) UpdateStatus(ctx context.Context, sessionID string, status models.UploadStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.UploadSession{}).
		Where("session_id = ?", sessionID).
		Update("status", status).Error
}

// UpdateProgress 更新上传进度
func (r *uploadSessionRepository) UpdateProgress(ctx context.Context, sessionID string, progress int) error {
	return r.db.WithContext(ctx).
		Model(&models.UploadSession{}).
		Where("session_id = ?", sessionID).
		Update("progress", progress).Error
}

// AddUploadedChunk 添加已上传分片
func (r *uploadSessionRepository) AddUploadedChunk(ctx context.Context, sessionID string, chunkIndex int) error {
	// 首先获取当前会话
	session, err := r.FindBySessionID(ctx, sessionID)
	if err != nil || session == nil {
		return errors.New("upload session not found")
	}

	// 添加分片
	session.AddUploadedChunk(chunkIndex)

	// 更新数据库
	return r.db.WithContext(ctx).
		Model(&models.UploadSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"uploaded_chunks": session.UploadedChunks,
			"progress":        session.Progress,
		}).Error
}

// MarkAsCompleted 标记上传完成
func (r *uploadSessionRepository) MarkAsCompleted(ctx context.Context, sessionID string, finalPath string, modelID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.UploadSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"status":            models.UploadStatusCompleted,
			"progress":          100,
			"final_path":        finalPath,
			"original_model_id": modelID,
			"completed_at":      &now,
		}).Error
}

// MarkAsFailed 标记上传失败
func (r *uploadSessionRepository) MarkAsFailed(ctx context.Context, sessionID string, errorMsg string) error {
	return r.db.WithContext(ctx).
		Model(&models.UploadSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"status":    models.UploadStatusFailed,
			"error_msg": errorMsg,
		}).Error
}

// CleanupExpiredSessions 清理过期的上传会话
func (r *uploadSessionRepository) CleanupExpiredSessions(ctx context.Context) error {
	// 标记过期会话
	err := r.db.WithContext(ctx).
		Model(&models.UploadSession{}).
		Where("expires_at < ? AND status IN (?)", time.Now(), []models.UploadStatus{
			models.UploadStatusInitialized,
			models.UploadStatusUploading,
		}).
		Update("status", models.UploadStatusExpired).Error

	if err != nil {
		return err
	}

	// 删除很久之前过期的会话记录
	oldTime := time.Now().AddDate(0, 0, -7) // 7天前
	return r.db.WithContext(ctx).
		Where("expires_at < ? AND status = ?", oldTime, models.UploadStatusExpired).
		Delete(&models.UploadSession{}).Error
}

// CleanupCompletedSessions 清理已完成的上传会话
func (r *uploadSessionRepository) CleanupCompletedSessions(ctx context.Context, olderThan time.Time) error {
	return r.db.WithContext(ctx).
		Where("completed_at IS NOT NULL AND completed_at < ? AND status = ?",
			olderThan, models.UploadStatusCompleted).
		Delete(&models.UploadSession{}).Error
}

// GetUploadStatistics 获取上传统计信息
func (r *uploadSessionRepository) GetUploadStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总会话数
	var totalCount int64
	if err := r.db.WithContext(ctx).Model(&models.UploadSession{}).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total"] = totalCount

	// 按状态统计
	statusStats := make(map[string]int64)
	type statusCount struct {
		Status models.UploadStatus `json:"status"`
		Count  int64               `json:"count"`
	}

	var statusResults []statusCount
	err := r.db.WithContext(ctx).
		Model(&models.UploadSession{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&statusResults).Error

	if err != nil {
		return nil, err
	}

	for _, result := range statusResults {
		statusStats[string(result.Status)] = result.Count
	}
	stats["by_status"] = statusStats

	// 活跃会话数
	var activeCount int64
	if err := r.db.WithContext(ctx).Model(&models.UploadSession{}).
		Where("status IN (?)", []models.UploadStatus{
			models.UploadStatusInitialized,
			models.UploadStatusUploading,
		}).
		Count(&activeCount).Error; err != nil {
		return nil, err
	}
	stats["active"] = activeCount

	// 今日上传数
	today := time.Now().Truncate(24 * time.Hour)
	var todayCount int64
	if err := r.db.WithContext(ctx).Model(&models.UploadSession{}).
		Where("created_at >= ? AND status = ?", today, models.UploadStatusCompleted).
		Count(&todayCount).Error; err != nil {
		return nil, err
	}
	stats["today_completed"] = todayCount

	// 平均上传大小
	type avgSize struct {
		AvgSize float64 `json:"avg_size"`
	}
	var size avgSize
	err = r.db.WithContext(ctx).
		Model(&models.UploadSession{}).
		Where("status = ?", models.UploadStatusCompleted).
		Select("AVG(file_size) as avg_size").
		Scan(&size).Error
	if err != nil {
		return nil, err
	}
	stats["avg_file_size"] = size.AvgSize

	// 成功率
	var successCount int64
	if err := r.db.WithContext(ctx).Model(&models.UploadSession{}).
		Where("status = ?", models.UploadStatusCompleted).
		Count(&successCount).Error; err != nil {
		return nil, err
	}

	if totalCount > 0 {
		stats["success_rate"] = float64(successCount) / float64(totalCount) * 100
	} else {
		stats["success_rate"] = 0.0
	}

	return stats, nil
}
