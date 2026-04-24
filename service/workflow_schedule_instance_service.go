package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// WorkflowScheduleInstanceService 调度实例服务接口
type WorkflowScheduleInstanceService interface {
	// CreateScheduleInstance 创建调度实例
	CreateScheduleInstance(ctx context.Context, scheduleID uint, triggerType string, triggerData map[string]interface{}, deploymentID uint) (*models.WorkflowScheduleInstance, error)

	// GetScheduleInstanceList 获取调度实例列表
	GetScheduleInstanceList(ctx context.Context, filter *repository.ScheduleInstanceFilter) ([]*models.WorkflowScheduleInstance, int64, error)

	// GetScheduleInstanceDetail 获取调度实例详情
	GetScheduleInstanceDetail(ctx context.Context, instanceID string) (*ScheduleInstanceDetail, error)

	// UpdateScheduleInstanceStatus 更新调度实例状态
	UpdateScheduleInstanceStatus(ctx context.Context, instanceID string, status string) error

	// AddWorkflowInstance 添加工作流实例到调度实例
	AddWorkflowInstance(ctx context.Context, scheduleInstanceID string, deploymentID uint, workflowInstanceID string) error

	// UpdateWorkflowInstanceStatus 更新工作流实例状态
	UpdateWorkflowInstanceStatus(ctx context.Context, scheduleInstanceID string, workflowInstanceID string, status string) error

	// GetScheduleInstanceStatistics 获取调度实例统计
	GetScheduleInstanceStatistics(ctx context.Context, scheduleID uint, startDate, endDate *time.Time) (*repository.ScheduleInstanceStatistics, error)
}

// ScheduleInstanceDetail 调度实例详情
type ScheduleInstanceDetail struct {
	*models.WorkflowScheduleInstance
	ScheduleName      string                    `json:"schedule_name"`
	WorkflowID        uint                      `json:"workflow_id"`
	WorkflowName      string                    `json:"workflow_name"`
	WorkflowInstances []WorkflowInstanceSummary `json:"workflow_instances"`
}

// WorkflowInstanceSummary 工作流实例摘要
type WorkflowInstanceSummary struct {
	DeploymentID   uint      `json:"deployment_id"`
	DeploymentName string    `json:"deployment_name"`
	DeploymentKey  string    `json:"deployment_key"`
	InstanceID     string    `json:"instance_id"`
	Status         string    `json:"status"`
	BoxID          uint      `json:"box_id"`
	BoxName        string    `json:"box_name"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time,omitempty"`
	Duration       int       `json:"duration"`
}

type workflowScheduleInstanceService struct {
	scheduleInstanceRepo repository.WorkflowScheduleInstanceRepository
	scheduleRepo         repository.WorkflowScheduleRepository
	workflowInstanceRepo repository.WorkflowInstanceRepository
	deploymentRepo       repository.WorkflowDeploymentRepository
}

// NewWorkflowScheduleInstanceService 创建调度实例服务
func NewWorkflowScheduleInstanceService(
	scheduleInstanceRepo repository.WorkflowScheduleInstanceRepository,
	scheduleRepo repository.WorkflowScheduleRepository,
	workflowInstanceRepo repository.WorkflowInstanceRepository,
	deploymentRepo repository.WorkflowDeploymentRepository,
) WorkflowScheduleInstanceService {
	return &workflowScheduleInstanceService{
		scheduleInstanceRepo: scheduleInstanceRepo,
		scheduleRepo:         scheduleRepo,
		workflowInstanceRepo: workflowInstanceRepo,
		deploymentRepo:       deploymentRepo,
	}
}

// CreateScheduleInstance 创建调度实例
func (s *workflowScheduleInstanceService) CreateScheduleInstance(ctx context.Context, scheduleID uint, triggerType string, triggerData map[string]interface{}, deploymentID uint) (*models.WorkflowScheduleInstance, error) {
	// 生成实例ID
	instanceID := fmt.Sprintf("schedule_inst_%s", uuid.New().String()[:8])

	// 转换触发数据为 JSONMap
	triggerDataMap := models.JSONMap(triggerData)

	now := time.Now()
	instance := &models.WorkflowScheduleInstance{
		ScheduleID:   scheduleID,
		InstanceID:   instanceID,
		TriggerType:  models.TriggerType(triggerType),
		TriggerTime:  now,
		TriggerData:  triggerDataMap,
		Status:       models.WorkflowScheduleInstanceStatusPending,
		DeploymentID: deploymentID,
		SuccessCount: 0,
		FailedCount:  0,
	}

	if err := s.scheduleInstanceRepo.Create(ctx, instance); err != nil {
		return nil, fmt.Errorf("failed to create schedule instance: %w", err)
	}

	return instance, nil
}

// GetScheduleInstanceList 获取调度实例列表
func (s *workflowScheduleInstanceService) GetScheduleInstanceList(ctx context.Context, filter *repository.ScheduleInstanceFilter) ([]*models.WorkflowScheduleInstance, int64, error) {
	return s.scheduleInstanceRepo.FindByFilter(ctx, filter)
}

// GetScheduleInstanceDetail 获取调度实例详情
func (s *workflowScheduleInstanceService) GetScheduleInstanceDetail(ctx context.Context, instanceID string) (*ScheduleInstanceDetail, error) {
	// 获取调度实例
	instance, err := s.scheduleInstanceRepo.FindByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to find schedule instance: %w", err)
	}

	// 获取调度配置
	schedule, err := s.scheduleRepo.GetByID(ctx, instance.ScheduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to find schedule: %w", err)
	}

	// 获取工作流实例详情
	workflowInstances := make([]WorkflowInstanceSummary, 0, len(instance.WorkflowInstanceIDs))
	for _, wfInst := range instance.WorkflowInstanceIDs {
		wfInstance, err := s.workflowInstanceRepo.FindByInstanceID(ctx, wfInst.InstanceID)
		if err != nil {
			continue
		}

		// 获取部署信息
		deployment, err := s.deploymentRepo.GetByID(ctx, wfInst.DeploymentID)
		if err != nil {
			continue
		}

		summary := WorkflowInstanceSummary{
			DeploymentID:   wfInst.DeploymentID,
			DeploymentName: deployment.Name,
			DeploymentKey:  deployment.Key,
			InstanceID:     wfInstance.InstanceID,
			Status:         string(wfInstance.Status),
			BoxID:          wfInstance.BoxID,
			Duration:       wfInstance.Duration,
		}

		if wfInstance.StartTime != nil {
			summary.StartTime = *wfInstance.StartTime
		}
		if wfInstance.EndTime != nil {
			summary.EndTime = *wfInstance.EndTime
		}

		workflowInstances = append(workflowInstances, summary)
	}

	detail := &ScheduleInstanceDetail{
		WorkflowScheduleInstance: instance,
		ScheduleName:             schedule.Name,
		WorkflowID:               schedule.WorkflowID,
		WorkflowInstances:        workflowInstances,
	}

	return detail, nil
}

// UpdateScheduleInstanceStatus 更新调度实例状态
func (s *workflowScheduleInstanceService) UpdateScheduleInstanceStatus(ctx context.Context, instanceID string, status string) error {
	return s.scheduleInstanceRepo.UpdateStatus(ctx, instanceID, status)
}

// AddWorkflowInstance 添加工作流实例到调度实例
func (s *workflowScheduleInstanceService) AddWorkflowInstance(ctx context.Context, scheduleInstanceID string, deploymentID uint, workflowInstanceID string) error {
	// 获取当前调度实例
	instance, err := s.scheduleInstanceRepo.FindByInstanceID(ctx, scheduleInstanceID)
	if err != nil {
		return fmt.Errorf("failed to find schedule instance: %w", err)
	}

	// 添加新的工作流实例
	workflowInstanceIDs := append(instance.WorkflowInstanceIDs, models.WorkflowInstanceIDMapping{
		DeploymentID: deploymentID,
		InstanceID:   workflowInstanceID,
	})

	// 更新调度实例
	if err := s.scheduleInstanceRepo.UpdateWorkflowInstanceIDs(ctx, scheduleInstanceID, workflowInstanceIDs); err != nil {
		return fmt.Errorf("failed to update workflow instance ids: %w", err)
	}

	// 更新调度实例状态为运行中
	if instance.Status == models.WorkflowScheduleInstanceStatusPending {
		now := time.Now()
		instance.Status = models.WorkflowScheduleInstanceStatusRunning
		instance.StartTime = &now
		if err := s.scheduleInstanceRepo.Update(ctx, instance); err != nil {
			return fmt.Errorf("failed to update schedule instance status: %w", err)
		}
	}

	return nil
}

// UpdateWorkflowInstanceStatus 更新工作流实例状态
func (s *workflowScheduleInstanceService) UpdateWorkflowInstanceStatus(ctx context.Context, scheduleInstanceID string, workflowInstanceID string, status string) error {
	// 获取调度实例
	instance, err := s.scheduleInstanceRepo.FindByInstanceID(ctx, scheduleInstanceID)
	if err != nil {
		return fmt.Errorf("failed to find schedule instance: %w", err)
	}

	// 统计成功和失败数量
	successCount := 0
	failedCount := 0
	allCompleted := true

	for _, wfInst := range instance.WorkflowInstanceIDs {
		wfInstance, err := s.workflowInstanceRepo.FindByInstanceID(ctx, wfInst.InstanceID)
		if err != nil {
			continue
		}

		switch wfInstance.Status {
		case models.WorkflowInstanceStatusCompleted:
			successCount++
		case models.WorkflowInstanceStatusFailed:
			failedCount++
		case models.WorkflowInstanceStatusRunning, models.WorkflowInstanceStatusPending:
			allCompleted = false
		}
	}

	// 更新计数
	if err := s.scheduleInstanceRepo.UpdateCounts(ctx, scheduleInstanceID, successCount, failedCount); err != nil {
		return fmt.Errorf("failed to update counts: %w", err)
	}

	// 如果所有工作流实例都已完成，更新调度实例状态
	if allCompleted && len(instance.WorkflowInstanceIDs) > 0 {
		var scheduleStatus string
		if failedCount == len(instance.WorkflowInstanceIDs) {
			scheduleStatus = string(models.WorkflowScheduleInstanceStatusFailed)
		} else {
			scheduleStatus = string(models.WorkflowScheduleInstanceStatusCompleted)
		}

		if err := s.scheduleInstanceRepo.UpdateStatus(ctx, scheduleInstanceID, scheduleStatus); err != nil {
			return fmt.Errorf("failed to update schedule instance status: %w", err)
		}
	}

	return nil
}

// GetScheduleInstanceStatistics 获取调度实例统计
func (s *workflowScheduleInstanceService) GetScheduleInstanceStatistics(ctx context.Context, scheduleID uint, startDate, endDate *time.Time) (*repository.ScheduleInstanceStatistics, error) {
	stats, err := s.scheduleInstanceRepo.GetStatistics(ctx, scheduleID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get statistics: %w", err)
	}

	// 获取调度名称
	schedule, err := s.scheduleRepo.GetByID(ctx, scheduleID)
	if err == nil {
		stats.ScheduleName = schedule.Name
	}

	return stats, nil
}
