package service

import (
	"box-manage-service/client"
	"box-manage-service/models"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const startupRecoveryRetryCooldown = 30 * time.Second

type startupRecoveryState struct {
	mu          sync.Mutex
	sessionID   string
	status      string
	message     string
	attempts    int
	lastAttempt time.Time
}

type startupRecoveryCounts struct {
	models    int
	workflows int
	schedules int
	tasks     int
}

func (s *BoxClientService) handleStartupRecovery(boxID uint, sessionID string) (bool, string, string) {
	stateValue, _ := s.recoveryStates.LoadOrStore(boxID, &startupRecoveryState{})
	state := stateValue.(*startupRecoveryState)

	state.mu.Lock()
	if state.sessionID != sessionID {
		if state.status == "running" {
			message := fmt.Sprintf("盒子已有恢复任务正在执行，完成后将处理当前会话: %s", state.sessionID)
			state.mu.Unlock()
			return true, "running", message
		}
		state.sessionID = sessionID
		state.status = ""
		state.message = ""
		state.attempts = 0
		state.lastAttempt = time.Time{}
	}

	now := time.Now()
	shouldStart := state.status == "" ||
		(state.status == "failed" && now.Sub(state.lastAttempt) >= startupRecoveryRetryCooldown)
	if shouldStart {
		state.status = "running"
		state.attempts++
		state.lastAttempt = now
		state.message = fmt.Sprintf("恢复任务已启动，第%d次尝试", state.attempts)
		attempt := state.attempts
		state.mu.Unlock()

		go s.runStartupRecovery(boxID, sessionID, attempt, state)
		return true, "running", fmt.Sprintf("恢复任务已启动，第%d次尝试", attempt)
	}

	status := state.status
	message := state.message
	state.mu.Unlock()
	return true, status, message
}

func (s *BoxClientService) runStartupRecovery(boxID uint, sessionID string, attempt int, state *startupRecoveryState) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Printf("[StartupRecovery] 开始恢复: BoxID=%d, Session=%s, Attempt=%d", boxID, sessionID, attempt)
	counts, err := s.restoreBoxConfiguration(ctx, boxID)

	state.mu.Lock()
	defer state.mu.Unlock()
	if state.sessionID != sessionID {
		log.Printf("[StartupRecovery] 会话已变化，丢弃旧恢复结果: BoxID=%d, Session=%s", boxID, sessionID)
		return
	}

	if err != nil {
		state.status = "failed"
		state.message = err.Error()
		log.Printf("[StartupRecovery] 恢复失败: BoxID=%d, Session=%s, Attempt=%d, Error=%v",
			boxID, sessionID, attempt, err)
		return
	}

	state.status = "completed"
	state.message = fmt.Sprintf("恢复完成: models=%d, workflows=%d, schedules=%d, tasks=%d",
		counts.models, counts.workflows, counts.schedules, counts.tasks)
	log.Printf("[StartupRecovery] %s, BoxID=%d, Session=%s", state.message, boxID, sessionID)
}

func (s *BoxClientService) restoreBoxConfiguration(ctx context.Context, boxID uint) (startupRecoveryCounts, error) {
	var counts startupRecoveryCounts
	if s.taskDeploymentService == nil {
		return counts, fmt.Errorf("任务部署服务未初始化")
	}

	box, err := s.repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		return counts, fmt.Errorf("获取盒子失败: %w", err)
	}
	boxClient := client.NewBoxClient(box)

	modelDeployments, err := s.repoManager.ModelBoxDeployment().GetByBoxID(ctx, boxID)
	if err != nil {
		return counts, fmt.Errorf("查询模型部署记录失败: %w", err)
	}
	for _, deployment := range modelDeployments {
		if deployment.Status == models.ModelBoxDeploymentStatusRemoved {
			continue
		}
		if err := s.taskDeploymentService.RestoreModelToBox(
			ctx, boxID, deployment.ConvertedModelID, deployment.ModelKey); err != nil {
			return counts, fmt.Errorf("恢复模型 %s 失败: %w", deployment.ModelKey, err)
		}
		counts.models++
	}
	log.Printf("[StartupRecovery] 模型恢复完成: BoxID=%d, Count=%d", boxID, counts.models)

	deployments, err := s.repoManager.WorkflowDeployment().FindByBoxID(ctx, boxID)
	if err != nil {
		return counts, fmt.Errorf("查询工作流部署记录失败: %w", err)
	}
	restoredDeploymentIDs := make([]uint, 0, len(deployments))
	for _, deployment := range deployments {
		if deployment.Status != models.DeploymentStatusDeployed {
			continue
		}
		workflow, err := s.repoManager.Workflow().GetByID(ctx, deployment.WorkflowID)
		if err != nil {
			return counts, fmt.Errorf("获取工作流 %d 失败: %w", deployment.WorkflowID, err)
		}
		nodes, err := s.repoManager.NodeDefinition().FindByWorkflowID(ctx, workflow.ID)
		if err != nil {
			return counts, fmt.Errorf("获取工作流 %d 节点失败: %w", workflow.ID, err)
		}
		variables, err := s.repoManager.VariableDefinition().FindByWorkflowID(ctx, workflow.ID)
		if err != nil {
			return counts, fmt.Errorf("获取工作流 %d 变量失败: %w", workflow.ID, err)
		}
		lines, err := s.repoManager.LineDefinition().FindByWorkflowID(ctx, workflow.ID)
		if err != nil {
			return counts, fmt.Errorf("获取工作流 %d 连接线失败: %w", workflow.ID, err)
		}

		payload := &client.DeploymentDistributionRequest{
			Deployment: deployment,
			Workflow:   workflow,
			Nodes:      convertToInterfaceSlice(nodes),
			Variables:  convertVariablesToBoxPayload(variables),
			Lines:      convertToInterfaceSlice(lines),
		}
		if err := boxClient.DistributeDeployment(ctx, payload); err != nil {
			return counts, fmt.Errorf("恢复工作流部署 %d 失败: %w", deployment.ID, err)
		}
		restoredDeploymentIDs = append(restoredDeploymentIDs, deployment.ID)
		counts.workflows++
	}
	log.Printf("[StartupRecovery] 工作流恢复完成: BoxID=%d, Count=%d", boxID, counts.workflows)

	restoredScheduleIDs := make(map[uint]struct{})
	for _, deploymentID := range restoredDeploymentIDs {
		schedules, err := s.repoManager.WorkflowSchedule().FindByDeploymentID(ctx, deploymentID)
		if err != nil {
			return counts, fmt.Errorf("查询部署 %d 的调度失败: %w", deploymentID, err)
		}
		for _, schedule := range schedules {
			if _, exists := restoredScheduleIDs[schedule.ID]; exists {
				continue
			}
			if err := boxClient.DistributeSchedule(ctx, schedule); err != nil {
				return counts, fmt.Errorf("恢复调度 %d 失败: %w", schedule.ID, err)
			}
			restoredScheduleIDs[schedule.ID] = struct{}{}
			counts.schedules++
		}
	}
	log.Printf("[StartupRecovery] 调度恢复完成: BoxID=%d, Count=%d", boxID, counts.schedules)

	tasks, err := s.repoManager.Task().FindByBoxID(ctx, boxID)
	if err != nil {
		return counts, fmt.Errorf("查询盒子任务失败: %w", err)
	}
	skipped := 0
	for _, task := range tasks {
		// 只恢复「用户希望它运行」的任务。
		//
		// 背景：原实现无条件重新下发全部任务，导致用户主动停止的任务在盒子重启后
		// 被恢复逻辑重新拉起。线上真实故障：一个断流任务（RTSP 已断 58 小时）
		// 每次重启都被恢复，持续重连把 box-app 拖到死锁、API 完全无响应。
		//
		// 判定规则：
		//   - 明确 cancelled / stopped 的跳过
		//   - 没有开启 auto_start 的也跳过（用户没要求开机自启）
		if !shouldRestoreTask(task) {
			skipped++
			log.Printf("[StartupRecovery] 跳过不恢复的任务: TaskID=%s, AutoStart=%v, RunStatus=%s, Status=%s",
				task.TaskID, task.AutoStart, task.RunStatus, task.Status)
			continue
		}
		response, err := s.taskDeploymentService.DeployTask(ctx, task.ID, boxID)
		if err != nil {
			return counts, fmt.Errorf("恢复任务 %s 失败: %w", task.TaskID, err)
		}
		if response == nil || !response.Success {
			message := "无部署响应"
			if response != nil && response.Message != "" {
				message = response.Message
			}
			return counts, fmt.Errorf("恢复任务 %s 失败: %s", task.TaskID, message)
		}
		counts.tasks++
	}
	if skipped > 0 {
		log.Printf("[StartupRecovery] 已跳过 %d 个不恢复的任务: BoxID=%d", skipped, boxID)
	}
	log.Printf("[StartupRecovery] 任务恢复完成: BoxID=%d, Count=%d", boxID, counts.tasks)

	return counts, nil
}

// shouldRestoreTask 判断盒子重启后是否应该恢复某个任务。
//
// 必须同时满足：
//  1. 任务不是「已取消」状态（用户明确停掉的）
//  2. 任务开启了 auto_start（用户希望开机自启）
//
// 这样用户主动停止的任务不会因为盒子重启被悄悄拉起。
func shouldRestoreTask(task *models.Task) bool {
	if task == nil {
		return false
	}
	// 用户明确取消的任务不恢复
	if task.Status == models.TaskStatusCancelled {
		return false
	}
	// 没有开启开机自启的任务不恢复
	if !task.AutoStart {
		return false
	}
	return true
}
