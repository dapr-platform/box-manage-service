/*
 * @module service/workflow_scheduler_service
 * @description 工作流调度服务实现
 * @architecture 业务逻辑层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow Scheduler -> WorkflowSchedulerService -> WorkflowInstanceService
 * @rules 实现工作流的定时调度和手动触发
 * @dependencies repository, models, service
 * @refs 业务编排引擎需求文档.md 5.8节
 */

package service

import (
	"box-manage-service/client"
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"time"
)

// WorkflowSchedulerService 工作流调度服务接口
type WorkflowSchedulerService interface {
	// 调度配置管理
	CreateSchedule(ctx context.Context, schedule *models.WorkflowSchedule) error
	UpdateSchedule(ctx context.Context, schedule *models.WorkflowSchedule) error
	DeleteSchedule(ctx context.Context, id uint) error
	GetSchedule(ctx context.Context, id uint) (*models.WorkflowSchedule, error)
	ListSchedules(ctx context.Context, workflowID uint) ([]*models.WorkflowSchedule, error)

	// 调度控制
	EnableSchedule(ctx context.Context, id uint) error
	DisableSchedule(ctx context.Context, id uint) error

	// 手动触发
	TriggerManual(ctx context.Context, scheduleID uint) error
}

// workflowSchedulerService 工作流调度服务实现
type workflowSchedulerService struct {
	scheduleRepo   repository.WorkflowScheduleRepository
	deploymentRepo repository.WorkflowDeploymentRepository
	boxRepo        repository.BoxRepository
	repoManager    repository.RepositoryManager
}

// NewWorkflowSchedulerService 创建工作流调度服务实例
func NewWorkflowSchedulerService(
	repoManager repository.RepositoryManager,
) WorkflowSchedulerService {
	return &workflowSchedulerService{
		scheduleRepo:   repository.NewWorkflowScheduleRepository(repoManager.DB()),
		deploymentRepo: repository.NewWorkflowDeploymentRepository(repoManager.DB()),
		boxRepo:        repoManager.Box(),
		repoManager:    repoManager,
	}
}

// CreateSchedule 创建调度配置
func (s *workflowSchedulerService) CreateSchedule(ctx context.Context, schedule *models.WorkflowSchedule) error {
	// 验证cron表达式
	if schedule.ScheduleType == models.ScheduleTypeCron {
		if schedule.CronExpression == "" {
			return fmt.Errorf("cron类型调度必须提供cron表达式")
		}
		// TODO: 可以添加cron表达式格式验证
	}

	// 创建调度配置
	if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
		return fmt.Errorf("创建调度配置失败: %w", err)
	}

	// 下发调度配置到盒子端
	if err := s.distributeScheduleToBoxes(ctx, schedule); err != nil {
		return fmt.Errorf("下发调度配置失败: %w", err)
	}

	return nil
}

// UpdateSchedule 更新调度配置
func (s *workflowSchedulerService) UpdateSchedule(ctx context.Context, schedule *models.WorkflowSchedule) error {
	// 验证cron表达式
	if schedule.ScheduleType == models.ScheduleTypeCron {
		if schedule.CronExpression == "" {
			return fmt.Errorf("cron类型调度必须提供cron表达式")
		}
	}

	// 更新调度配置
	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		return fmt.Errorf("更新调度配置失败: %w", err)
	}

	// 重新下发调度配置到盒子端
	if err := s.distributeScheduleToBoxes(ctx, schedule); err != nil {
		return fmt.Errorf("下发调度配置失败: %w", err)
	}

	return nil
}

// DeleteSchedule 删除调度配置
func (s *workflowSchedulerService) DeleteSchedule(ctx context.Context, id uint) error {
	// 获取调度配置
	schedule, err := s.scheduleRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取调度配置失败: %w", err)
	}

	// 通知盒子端删除调度
	if err := s.deleteScheduleFromBoxes(ctx, schedule); err != nil {
		// 记录错误但继续删除服务端配置
		fmt.Printf("通知盒子端删除调度失败: %v\n", err)
	}

	// 删除调度配置
	return s.scheduleRepo.Delete(ctx, id)
}

// GetSchedule 获取调度配置
func (s *workflowSchedulerService) GetSchedule(ctx context.Context, id uint) (*models.WorkflowSchedule, error) {
	return s.scheduleRepo.GetByID(ctx, id)
}

// ListSchedules 列出调度配置
func (s *workflowSchedulerService) ListSchedules(ctx context.Context, workflowID uint) ([]*models.WorkflowSchedule, error) {
	return s.scheduleRepo.FindByWorkflowID(ctx, workflowID)
}

// EnableSchedule 启用调度配置
func (s *workflowSchedulerService) EnableSchedule(ctx context.Context, id uint) error {
	schedule, err := s.scheduleRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.scheduleRepo.Enable(ctx, id); err != nil {
		return err
	}

	// 通知盒子端启用调度
	schedule.IsEnabled = true
	if err := s.distributeScheduleToBoxes(ctx, schedule); err != nil {
		return fmt.Errorf("通知盒子端启用调度失败: %w", err)
	}

	return nil
}

// DisableSchedule 禁用调度配置
func (s *workflowSchedulerService) DisableSchedule(ctx context.Context, id uint) error {
	schedule, err := s.scheduleRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.scheduleRepo.Disable(ctx, id); err != nil {
		return err
	}

	// 通知盒子端禁用调度
	schedule.IsEnabled = false
	if err := s.distributeScheduleToBoxes(ctx, schedule); err != nil {
		return fmt.Errorf("通知盒子端禁用调度失败: %w", err)
	}

	return nil
}

// TriggerManual 手动触发
func (s *workflowSchedulerService) TriggerManual(ctx context.Context, scheduleID uint) error {
	schedule, err := s.scheduleRepo.GetByID(ctx, scheduleID)
	if err != nil {
		return fmt.Errorf("获取调度配置失败: %w", err)
	}

	// 通知盒子端手动触发调度
	// TODO: 实现手动触发的盒子端API调用
	// 目前盒子端可能需要添加手动触发的API端点

	// 更新最后运行时间
	now := time.Now()
	s.scheduleRepo.UpdateLastRunTime(ctx, schedule.ID, now)
	s.scheduleRepo.IncrementRunCount(ctx, schedule.ID)

	return fmt.Errorf("手动触发功能需要盒子端支持")
}

// distributeScheduleToBoxes 将调度配置下发到盒子端
func (s *workflowSchedulerService) distributeScheduleToBoxes(ctx context.Context, schedule *models.WorkflowSchedule) error {
	// 遍历所有部署ID
	for _, deploymentID := range schedule.DeploymentIDs {
		// 获取部署信息
		deployment, err := s.deploymentRepo.GetByID(ctx, deploymentID)
		if err != nil {
			fmt.Printf("获取部署信息失败 (DeploymentID: %d): %v\n", deploymentID, err)
			continue
		}

		// 获取盒子信息
		box, err := s.boxRepo.GetByID(ctx, deployment.BoxID)
		if err != nil {
			fmt.Printf("获取盒子信息失败 (BoxID: %d): %v\n", deployment.BoxID, err)
			continue
		}

		// 检查盒子是否在线
		if box.Status != models.BoxStatusOnline {
			fmt.Printf("盒子不在线，跳过下发 (BoxID: %d, Status: %s)\n", box.ID, box.Status)
			continue
		}

		// 创建盒子客户端
		boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))

		// 直接下发调度配置（使用模型定义）
		if err := boxClient.DistributeSchedule(ctx, schedule); err != nil {
			fmt.Printf("下发调度配置失败 (BoxID: %d, ScheduleID: %d): %v\n", box.ID, schedule.ID, err)
			continue
		}

		fmt.Printf("调度配置下发成功 (BoxID: %d, ScheduleID: %d)\n", box.ID, schedule.ID)
	}

	return nil
}

// deleteScheduleFromBoxes 通知盒子端删除调度
func (s *workflowSchedulerService) deleteScheduleFromBoxes(ctx context.Context, schedule *models.WorkflowSchedule) error {
	// 遍历所有部署ID
	for _, deploymentID := range schedule.DeploymentIDs {
		// 获取部署信息
		deployment, err := s.deploymentRepo.GetByID(ctx, deploymentID)
		if err != nil {
			fmt.Printf("获取部署信息失败 (DeploymentID: %d): %v\n", deploymentID, err)
			continue
		}

		// 获取盒子信息
		box, err := s.boxRepo.GetByID(ctx, deployment.BoxID)
		if err != nil {
			fmt.Printf("获取盒子信息失败 (BoxID: %d): %v\n", deployment.BoxID, err)
			continue
		}

		// 检查盒子是否在线
		if box.Status != models.BoxStatusOnline {
			fmt.Printf("盒子不在线，跳过删除 (BoxID: %d, Status: %s)\n", box.ID, box.Status)
			continue
		}

		// 创建盒子客户端
		boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))

		// 删除调度配置
		scheduleID := fmt.Sprintf("%d", schedule.ID)
		if err := boxClient.DeleteSchedule(ctx, scheduleID); err != nil {
			fmt.Printf("删除调度配置失败 (BoxID: %d, ScheduleID: %d): %v\n", box.ID, schedule.ID, err)
			continue
		}

		fmt.Printf("调度配置删除成功 (BoxID: %d, ScheduleID: %d)\n", box.ID, schedule.ID)
	}

	return nil
}
