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

// WorkflowScheduleDetail 带关联工作流对象的调度详情
type WorkflowScheduleDetail struct {
	*models.WorkflowSchedule
	Workflow *models.Workflow `json:"workflow,omitempty"` // 关联的工作流对象（含节点和变量）
}

// WorkflowSchedulerService 工作流调度服务接口
type WorkflowSchedulerService interface {
	// 调度配置管理
	CreateSchedule(ctx context.Context, schedule *models.WorkflowSchedule) error
	UpdateSchedule(ctx context.Context, schedule *models.WorkflowSchedule) error
	DeleteSchedule(ctx context.Context, id uint) error
	GetSchedule(ctx context.Context, id uint) (*WorkflowScheduleDetail, error)
	ListSchedules(ctx context.Context, workflowID uint) ([]*WorkflowScheduleDetail, error)
	ListSchedulesWithFilter(ctx context.Context, filter *repository.ScheduleFilter) ([]*WorkflowScheduleDetail, int64, error)

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
	workflowRepo   repository.WorkflowRepository
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
		workflowRepo:   repoManager.Workflow(),
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
func (s *workflowSchedulerService) GetSchedule(ctx context.Context, id uint) (*WorkflowScheduleDetail, error) {
	schedule, err := s.scheduleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	details := s.enrichSchedulesWithWorkflow(ctx, []*models.WorkflowSchedule{schedule})
	if len(details) == 0 {
		return nil, fmt.Errorf("调度不存在")
	}
	return details[0], nil
}

// ListSchedules 列出调度配置
func (s *workflowSchedulerService) ListSchedules(ctx context.Context, workflowID uint) ([]*WorkflowScheduleDetail, error) {
	schedules, err := s.scheduleRepo.FindByWorkflowID(ctx, workflowID)
	if err != nil {
		return nil, err
	}
	return s.enrichSchedulesWithWorkflow(ctx, schedules), nil
}

// ListSchedulesWithFilter 过滤分页查询调度配置
func (s *workflowSchedulerService) ListSchedulesWithFilter(ctx context.Context, filter *repository.ScheduleFilter) ([]*WorkflowScheduleDetail, int64, error) {
	schedules, total, err := s.scheduleRepo.FindByFilter(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	return s.enrichSchedulesWithWorkflow(ctx, schedules), total, nil
}

// enrichSchedulesWithWorkflow 批量加载调度关联的工作流对象
func (s *workflowSchedulerService) enrichSchedulesWithWorkflow(ctx context.Context, schedules []*models.WorkflowSchedule) []*WorkflowScheduleDetail {
	if len(schedules) == 0 {
		return []*WorkflowScheduleDetail{}
	}

	// 使用 map 缓存避免重复查询
	workflowCache := map[uint]*models.Workflow{}

	for _, sch := range schedules {
		if sch.WorkflowID > 0 {
			if _, ok := workflowCache[sch.WorkflowID]; !ok {
				if wf, err := s.workflowRepo.GetByID(ctx, sch.WorkflowID); err == nil {
					// 解析结构JSON以填充 Nodes, Lines, Variables
					wf.ParseStructureJSON()
					// 清空大字段
					wf.StructureJSON = ""
					wf.StructureJSONView = ""
					workflowCache[sch.WorkflowID] = wf
				}
			}
		}
	}

	details := make([]*WorkflowScheduleDetail, 0, len(schedules))
	for _, sch := range schedules {
		d := &WorkflowScheduleDetail{
			WorkflowSchedule: sch,
		}
		if wf, ok := workflowCache[sch.WorkflowID]; ok {
			d.Workflow = wf
		}
		details = append(details, d)
	}
	return details
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
	// 获取部署信息
	deployment, err := s.deploymentRepo.GetByID(ctx, schedule.DeploymentID)
	if err != nil {
		return fmt.Errorf("获取部署信息失败 (DeploymentID: %d): %w", schedule.DeploymentID, err)
	}

	// 获取盒子信息
	box, err := s.boxRepo.GetByID(ctx, deployment.BoxID)
	if err != nil {
		return fmt.Errorf("获取盒子信息失败 (BoxID: %d): %w", deployment.BoxID, err)
	}

	// 检查盒子是否在线
	if box.Status != models.BoxStatusOnline {
		fmt.Printf("[Scheduler] 盒子不在线，跳过下发 (BoxID: %d, Status: %s)\n", box.ID, box.Status)
		return nil
	}

	fmt.Printf("[Scheduler] 下发调度配置: ScheduleID=%d, DeploymentID=%d, BoxID=%d, BoxIP=%s\n",
		schedule.ID, schedule.DeploymentID, box.ID, box.IPAddress)

	// 创建盒子客户端并下发调度配置
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))
	if err := boxClient.DistributeSchedule(ctx, schedule); err != nil {
		return fmt.Errorf("下发调度配置失败 (BoxID: %d, ScheduleID: %d): %w", box.ID, schedule.ID, err)
	}

	fmt.Printf("[Scheduler] ✅ 调度配置下发成功 (BoxID: %d, ScheduleID: %d)\n", box.ID, schedule.ID)
	return nil
}

// deleteScheduleFromBoxes 通知盒子端删除调度
func (s *workflowSchedulerService) deleteScheduleFromBoxes(ctx context.Context, schedule *models.WorkflowSchedule) error {
	// 获取部署信息
	deployment, err := s.deploymentRepo.GetByID(ctx, schedule.DeploymentID)
	if err != nil {
		fmt.Printf("获取部署信息失败 (DeploymentID: %d): %v\n", schedule.DeploymentID, err)
		return nil
	}

	// 获取盒子信息
	box, err := s.boxRepo.GetByID(ctx, deployment.BoxID)
	if err != nil {
		fmt.Printf("获取盒子信息失败 (BoxID: %d): %v\n", deployment.BoxID, err)
		return nil
	}

	// 检查盒子是否在线
	if box.Status != models.BoxStatusOnline {
		fmt.Printf("盒子不在线，跳过删除 (BoxID: %d, Status: %s)\n", box.ID, box.Status)
		return nil
	}

	// 创建盒子客户端并删除调度配置
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))
	scheduleID := fmt.Sprintf("%d", schedule.ID)
	if err := boxClient.DeleteSchedule(ctx, scheduleID); err != nil {
		fmt.Printf("删除调度配置失败 (BoxID: %d, ScheduleID: %d): %v\n", box.ID, schedule.ID, err)
		return nil
	}

	fmt.Printf("调度配置删除成功 (BoxID: %d, ScheduleID: %d)\n", box.ID, schedule.ID)
	return nil
}
