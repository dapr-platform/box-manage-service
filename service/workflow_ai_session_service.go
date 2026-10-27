/*
 * @module service/workflow_ai_session_service
 * @description AI 编排会话的保存与读取。
 *
 * 目标：把「与 AI 对话式编排」的过程持久化，使关闭弹窗或刷新页面后
 * 仍能在同一份编排上继续修改，而不是每次从零重新提问生成。
 *
 * 设计取舍：
 * - 会话按 workflow_key_name 关联（不是自增 ID），编排升级版本后会话仍有效。
 * - 只在**用户主动保存**时落库，AI 生成过程本身不产生垃圾数据。
 * - latest_graph 存最近一次产出的完整编排，下次打开可直接接着改，不必重跑大模型。
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 会话状态
const (
	// AI 会话进行中
	AISessionStatusOpen = "open"
	// 生成的编排已导入画布
	AISessionStatusImported = "imported"
	// 用户主动结束
	AISessionStatusClosed = "closed"
)

// 会话标题截取长度
const aiSessionTitleLimit = 40

// WorkflowAISessionService AI 编排会话服务接口
type WorkflowAISessionService interface {
	// Save 保存/更新一次会话（按 workflow_key_name 定位，已存在则更新最近一条）
	Save(ctx context.Context, session *models.WorkflowAISession) (*models.WorkflowAISession, error)
	// ListByWorkflowKeyName 查询该编排的全部会话
	ListByWorkflowKeyName(ctx context.Context, keyName string) ([]*models.WorkflowAISession, error)
	// Latest 取该编排最近一次会话，没有则返回 nil
	Latest(ctx context.Context, keyName string) (*models.WorkflowAISession, error)
	// Delete 删除会话
	Delete(ctx context.Context, id uint) error
}

type workflowAISessionService struct {
	sessionRepo  repository.WorkflowAISessionRepository
	workflowRepo repository.WorkflowRepository
}

// NewWorkflowAISessionService 创建 AI 会话服务
func NewWorkflowAISessionService(repoManager repository.RepositoryManager) WorkflowAISessionService {
	service := &workflowAISessionService{}
	if repoManager == nil {
		return service
	}
	service.sessionRepo = repoManager.WorkflowAISession()
	service.workflowRepo = repoManager.Workflow()
	return service
}

// Save 保存会话。已存在同 key_name 的会话时更新最近一条，否则新建。
func (s *workflowAISessionService) Save(ctx context.Context, session *models.WorkflowAISession) (*models.WorkflowAISession, error) {
	if s.sessionRepo == nil {
		return nil, errors.New("AI 会话服务未正确初始化")
	}
	if session == nil {
		return nil, errors.New("会话内容不能为空")
	}
	if err := s.resolveKeyName(ctx, session); err != nil {
		return nil, err
	}
	s.normalize(session)

	existing, err := s.sessionRepo.FindLatestByWorkflowKeyName(ctx, session.WorkflowKeyName)
	if err != nil {
		return nil, fmt.Errorf("查询已有会话失败: %w", err)
	}
	if existing != nil {
		// 同一编排的连续对话复用同一条会话，避免每次保存都新增一行
		session.ID = existing.ID
		if session.Status == "" {
			session.Status = existing.Status
		}
		if err := s.sessionRepo.Update(ctx, session); err != nil {
			return nil, fmt.Errorf("更新会话失败: %w", err)
		}
		return session, nil
	}
	if session.Status == "" {
		session.Status = AISessionStatusOpen
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("创建会话失败: %w", err)
	}
	return session, nil
}

// ListByWorkflowKeyName 查询该编排的会话列表
func (s *workflowAISessionService) ListByWorkflowKeyName(ctx context.Context, keyName string) ([]*models.WorkflowAISession, error) {
	if s.sessionRepo == nil {
		return nil, errors.New("AI 会话服务未正确初始化")
	}
	keyName = strings.TrimSpace(keyName)
	if keyName == "" {
		return nil, errors.New("编排标识不能为空")
	}
	return s.sessionRepo.FindByWorkflowKeyName(ctx, keyName)
}

// Latest 取最近一次会话
func (s *workflowAISessionService) Latest(ctx context.Context, keyName string) (*models.WorkflowAISession, error) {
	if s.sessionRepo == nil {
		return nil, errors.New("AI 会话服务未正确初始化")
	}
	keyName = strings.TrimSpace(keyName)
	if keyName == "" {
		return nil, errors.New("编排标识不能为空")
	}
	return s.sessionRepo.FindLatestByWorkflowKeyName(ctx, keyName)
}

// Delete 删除会话
func (s *workflowAISessionService) Delete(ctx context.Context, id uint) error {
	if s.sessionRepo == nil {
		return errors.New("AI 会话服务未正确初始化")
	}
	if id == 0 {
		return errors.New("会话 ID 无效")
	}
	return s.sessionRepo.Delete(ctx, id)
}

// resolveKeyName 补全 workflow_key_name：优先用入参，否则按 workflow_id 反查。
func (s *workflowAISessionService) resolveKeyName(ctx context.Context, session *models.WorkflowAISession) error {
	session.WorkflowKeyName = strings.TrimSpace(session.WorkflowKeyName)
	if session.WorkflowKeyName != "" {
		return nil
	}
	if session.WorkflowID == 0 {
		return errors.New("workflow_key_name 与 workflow_id 至少提供一个")
	}
	if s.workflowRepo == nil {
		return errors.New("无法根据 workflow_id 反查编排标识")
	}
	workflow, err := s.workflowRepo.GetByID(ctx, session.WorkflowID)
	if err != nil {
		return fmt.Errorf("查询编排失败: %w", err)
	}
	if workflow == nil {
		return fmt.Errorf("编排 %d 不存在", session.WorkflowID)
	}
	session.WorkflowKeyName = workflow.KeyName
	if session.WorkflowID == 0 {
		session.WorkflowID = workflow.ID
	}
	return nil
}

// normalize 补齐标题、轮次、时间戳等派生字段。
func (s *workflowAISessionService) normalize(session *models.WorkflowAISession) {
	// 标题：优先用入参，否则取需求或首条用户消息
	if strings.TrimSpace(session.Title) == "" {
		candidate := strings.TrimSpace(session.Description)
		if candidate == "" {
			for _, raw := range session.Messages {
				message, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if asString(message["role"]) != "user" {
					continue
				}
				candidate = strings.TrimSpace(asString(message["content"]))
				break
			}
		}
		if candidate != "" {
			session.Title = truncateRunes(candidate, aiSessionTitleLimit)
		}
	}
	// 轮次：以消息里最大的 round 为准
	maxRound := session.Round
	now := time.Now().Format("2006-01-02 15:04:05")
	for _, raw := range session.Messages {
		message, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if round, ok := toFloat(message["round"]); ok && int(round) > maxRound {
			maxRound = int(round)
		}
		if asString(message["created_at"]) == "" {
			message["created_at"] = now
		}
	}
	session.Round = maxRound
	if session.Status == "" {
		session.Status = AISessionStatusOpen
	}
}

// truncateRunes 按字符（而非字节）截断，避免截断中文出现乱码。
func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

// DecodeAISessionMessages 把库里的 JSON 数组转成结构化消息，供上层使用。
func DecodeAISessionMessages(session *models.WorkflowAISession) ([]models.WorkflowAISessionMessage, error) {
	if session == nil || len(session.Messages) == 0 {
		return []models.WorkflowAISessionMessage{}, nil
	}
	raw, err := json.Marshal(session.Messages)
	if err != nil {
		return nil, err
	}
	var messages []models.WorkflowAISessionMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}
