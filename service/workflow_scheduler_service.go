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
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
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

	// 调度器管理
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// workflowSchedulerService 工作流调度服务实现
type workflowSchedulerService struct {
	scheduleRepo    repository.WorkflowScheduleRepository
	instanceService WorkflowInstanceService
	executorService WorkflowExecutorService
	cron            *cron.Cron
	cronEntries     map[uint]cron.EntryID // schedule_id -> cron_entry_id
}

// NewWorkflowSchedulerService 创建工作流调度服务实例
func NewWorkflowSchedulerService(
	repoManager repository.RepositoryManager,
	instanceService WorkflowInstanceService,
	executorService WorkflowExecutorService,
) WorkflowSchedulerService {
	return &workflowSchedulerService{
		scheduleRepo:    repository.NewWorkflowScheduleRepository(repoManager.DB()),
		instanceService: instanceService,
		executorService: executorService,
		cron:            cron.New(),
		cronEntries:     make(map[uint]cron.EntryID),
	}
}

// CreateSchedule 创建调度配置
func (s *workflowSchedulerService) CreateSchedule(ctx context.Context, schedule *models.WorkflowSchedule) error {
	// 验证cron表达式
	if schedule.ScheduleType == models.ScheduleTypeCron {
		if _, err := cron.ParseStandard(schedule.CronExpression); err != nil {
			return fmt.Errorf("无效的cron表达式: %w", err)
		}
	}

	// 创建调度配置
	if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
		return fmt.Errorf("创建调度配置失败: %w", err)
	}

	// 如果是启用状态且是cron类型，添加到调度器
	if schedule.IsEnabled && schedule.ScheduleType == models.ScheduleTypeCron {
		s.addToCron(schedule)
	}

	return nil
}

// UpdateSchedule 更新调度配置
func (s *workflowSchedulerService) UpdateSchedule(ctx context.Context, schedule *models.WorkflowSchedule) error {
	// 验证cron表达式
	if schedule.ScheduleType == models.ScheduleTypeCron {
		if _, err := cron.ParseStandard(schedule.CronExpression); err != nil {
			return fmt.Errorf("无效的cron表达式: %w", err)
		}
	}

	// 从调度器中移除旧的任务
	s.removeFromCron(schedule.ID)

	// 更新调度配置
	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		return fmt.Errorf("更新调度配置失败: %w", err)
	}

	// 如果是启用状态且是cron类型，重新添加到调度器
	if schedule.IsEnabled && schedule.ScheduleType == models.ScheduleTypeCron {
		s.addToCron(schedule)
	}

	return nil
}

// DeleteSchedule 删除调度配置
func (s *workflowSchedulerService) DeleteSchedule(ctx context.Context, id uint) error {
	// 从调度器中移除
	s.removeFromCron(id)

	// 删除调度配置
	return s.scheduleRepo.SoftDelete(ctx, id)
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

	// 如果是cron类型，添加到调度器
	if schedule.ScheduleType == models.ScheduleTypeCron {
		schedule.IsEnabled = true
		s.addToCron(schedule)
	}

	return nil
}

// DisableSchedule 禁用调度配置
func (s *workflowSchedulerService) DisableSchedule(ctx context.Context, id uint) error {
	// 从调度器中移除
	s.removeFromCron(id)

	return s.scheduleRepo.Disable(ctx, id)
}

// TriggerManual 手动触发
func (s *workflowSchedulerService) TriggerManual(ctx context.Context, scheduleID uint) error {
	schedule, err := s.scheduleRepo.GetByID(ctx, scheduleID)
	if err != nil {
		return fmt.Errorf("获取调度配置失败: %w", err)
	}

	return s.executeSchedule(ctx, schedule)
}

// Start 启动调度器
func (s *workflowSchedulerService) Start(ctx context.Context) error {
	// 加载所有启用的cron调度
	schedules, err := s.scheduleRepo.FindEnabled(ctx)
	if err != nil {
		return fmt.Errorf("加载调度配置失败: %w", err)
	}

	for _, schedule := range schedules {
		if schedule.ScheduleType == models.ScheduleTypeCron {
			s.addToCron(schedule)
		}
	}

	// 启动cron调度器
	s.cron.Start()

	return nil
}

// Stop 停止调度器
func (s *workflowSchedulerService) Stop(ctx context.Context) error {
	s.cron.Stop()
	return nil
}

// addToCron 添加到cron调度器
func (s *workflowSchedulerService) addToCron(schedule *models.WorkflowSchedule) {
	entryID, err := s.cron.AddFunc(schedule.CronExpression, func() {
		ctx := context.Background()
		s.executeSchedule(ctx, schedule)
	})

	if err == nil {
		s.cronEntries[schedule.ID] = entryID
	}
}

// removeFromCron 从cron调度器中移除
func (s *workflowSchedulerService) removeFromCron(scheduleID uint) {
	if entryID, ok := s.cronEntries[scheduleID]; ok {
		s.cron.Remove(entryID)
		delete(s.cronEntries, scheduleID)
	}
}

// executeSchedule 执行调度
func (s *workflowSchedulerService) executeSchedule(ctx context.Context, schedule *models.WorkflowSchedule) error {
	// 创建工作流实例
	inputVariables := make(map[string]interface{})
	if schedule.InputVariables.Variables != nil {
		inputVariables = schedule.InputVariables.Variables
	}

	instance, err := s.instanceService.CreateFromWorkflow(
		ctx,
		schedule.WorkflowID,
		nil,
		inputVariables,
		fmt.Sprintf("schedule:%d", schedule.ID),
	)
	if err != nil {
		return fmt.Errorf("创建工作流实例失败: %w", err)
	}

	// 执行工作流实例
	go func() {
		execCtx := context.Background()
		s.executorService.Execute(execCtx, instance.ID)
	}()

	// 更新调度配置
	now := time.Now()
	s.scheduleRepo.UpdateLastRunTime(ctx, schedule.ID, now)
	s.scheduleRepo.IncrementRunCount(ctx, schedule.ID)

	// 计算下次执行时间
	if schedule.ScheduleType == models.ScheduleTypeCron {
		cronSchedule, err := cron.ParseStandard(schedule.CronExpression)
		if err == nil {
			nextTime := cronSchedule.Next(now)
			s.scheduleRepo.UpdateNextRunTime(ctx, schedule.ID, nextTime)
		}
	}

	return nil
}
