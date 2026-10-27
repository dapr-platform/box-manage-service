/*
 * @module repository/workflow_ai_session_repository
 * @description AI 编排会话数据访问层
 */

package repository

import (
	"box-manage-service/models"
	"context"

	"gorm.io/gorm"
)

// WorkflowAISessionRepository AI 编排会话 Repository 接口
type WorkflowAISessionRepository interface {
	BaseRepository[models.WorkflowAISession]

	// FindByWorkflowKeyName 按编排标识查询会话（跨版本复用），按更新时间倒序
	FindByWorkflowKeyName(ctx context.Context, keyName string) ([]*models.WorkflowAISession, error)
	// FindLatestByWorkflowKeyName 取该编排最近一次的会话，没有则返回 nil
	FindLatestByWorkflowKeyName(ctx context.Context, keyName string) (*models.WorkflowAISession, error)
}

type workflowAISessionRepository struct {
	BaseRepository[models.WorkflowAISession]
	db *gorm.DB
}

// NewWorkflowAISessionRepository 创建 AI 会话 Repository
func NewWorkflowAISessionRepository(db *gorm.DB) WorkflowAISessionRepository {
	return &workflowAISessionRepository{
		BaseRepository: newBaseRepository[models.WorkflowAISession](db),
		db:             db,
	}
}

// FindByWorkflowKeyName 按编排标识查询会话
func (r *workflowAISessionRepository) FindByWorkflowKeyName(ctx context.Context, keyName string) ([]*models.WorkflowAISession, error) {
	var sessions []*models.WorkflowAISession
	if err := r.db.WithContext(ctx).
		Where("workflow_key_name = ?", keyName).
		Order("updated_at DESC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// FindLatestByWorkflowKeyName 取最近一次会话
func (r *workflowAISessionRepository) FindLatestByWorkflowKeyName(ctx context.Context, keyName string) (*models.WorkflowAISession, error) {
	var session models.WorkflowAISession
	err := r.db.WithContext(ctx).
		Where("workflow_key_name = ?", keyName).
		Order("updated_at DESC").
		First(&session).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}
