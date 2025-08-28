package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// deploymentTaskService 部署任务服务实现
type deploymentTaskService struct {
	deploymentRepo    repository.DeploymentRepository
	taskRepo          repository.TaskRepository
	taskDeploymentSvc TaskDeploymentService
	mu                sync.RWMutex
	runningTasks      map[uint]*deploymentExecution // 运行中的部署任务
}

// deploymentExecution 部署执行上下文
type deploymentExecution struct {
	deploymentTask *models.DeploymentTask
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
}

// NewDeploymentTaskService 创建部署任务服务
func NewDeploymentTaskService(
	deploymentRepo repository.DeploymentRepository,
	taskRepo repository.TaskRepository,
	taskDeploymentSvc TaskDeploymentService,
) DeploymentTaskService {
	service := &deploymentTaskService{
		deploymentRepo:    deploymentRepo,
		taskRepo:          taskRepo,
		taskDeploymentSvc: taskDeploymentSvc,
		runningTasks:      make(map[uint]*deploymentExecution),
	}

	// 启动时恢复运行中的任务
	go service.recoverRunningTasks()

	return service
}

// CreateDeploymentTask 创建部署任务
func (s *deploymentTaskService) CreateDeploymentTask(ctx context.Context, req *CreateDeploymentTaskRequest) (*models.DeploymentTask, error) {
	log.Printf("[DeploymentTaskService] Creating deployment task: %s", req.Name)

	// 验证任务和盒子ID
	if err := s.validateTasksAndBoxes(ctx, req.TaskIDs, req.TargetBoxIDs); err != nil {
		return nil, fmt.Errorf("验证任务和盒子失败: %w", err)
	}

	// 设置默认配置
	config := req.DeploymentConfig
	if config == nil {
		config = &models.DeploymentConfig{
			ConcurrentLimit:      5,
			RetryAttempts:        3,
			TimeoutSeconds:       300,
			StopOnFirstFailure:   false,
			ValidateBeforeDeploy: true,
			CleanupOnFailure:     true,
		}
	}

	// 创建部署任务
	deploymentTask := &models.DeploymentTask{
		Name:             req.Name,
		Description:      req.Description,
		Status:           models.DeploymentTaskStatusPending,
		Priority:         req.Priority,
		TaskIDs:          models.UintArray(req.TaskIDs),
		TargetBoxIDs:     models.UintArray(req.TargetBoxIDs),
		DeploymentConfig: *config,
		TotalTasks:       len(req.TaskIDs) * len(req.TargetBoxIDs),
		CreatedBy:        req.CreatedBy,
	}

	// 估算执行时间
	estimatedTime := time.Duration(deploymentTask.TotalTasks) * 30 * time.Second // 每个部署平均30秒
	deploymentTask.EstimatedTime = &estimatedTime

	// 保存到数据库
	if err := s.deploymentRepo.Create(ctx, deploymentTask); err != nil {
		return nil, fmt.Errorf("保存部署任务失败: %w", err)
	}

	deploymentTask.AddLog("INFO", "部署任务创建成功", nil, nil, map[string]interface{}{
		"total_tasks": deploymentTask.TotalTasks,
		"task_ids":    req.TaskIDs,
		"box_ids":     req.TargetBoxIDs,
	})

	log.Printf("[DeploymentTaskService] Deployment task created: ID=%d, TotalTasks=%d",
		deploymentTask.ID, deploymentTask.TotalTasks)

	return deploymentTask, nil
}

// GetDeploymentTask 获取部署任务
func (s *deploymentTaskService) GetDeploymentTask(ctx context.Context, id uint) (*models.DeploymentTask, error) {
	return s.deploymentRepo.GetByID(ctx, id)
}

// GetDeploymentTasks 获取部署任务列表
func (s *deploymentTaskService) GetDeploymentTasks(ctx context.Context, filters *DeploymentTaskFilters) ([]*models.DeploymentTask, error) {
	// TODO: 根据过滤条件查询
	// 这里简化实现，直接返回所有任务
	return s.deploymentRepo.Find(ctx, nil)
}

// UpdateDeploymentTask 更新部署任务
func (s *deploymentTaskService) UpdateDeploymentTask(ctx context.Context, id uint, updates map[string]interface{}) error {
	deploymentTask, err := s.deploymentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 检查是否可以更新
	if deploymentTask.IsRunning() {
		return fmt.Errorf("运行中的部署任务不能更新")
	}

	// 应用更新
	// TODO: 实现字段更新逻辑

	return s.deploymentRepo.Update(ctx, deploymentTask)
}

// DeleteDeploymentTask 删除部署任务
func (s *deploymentTaskService) DeleteDeploymentTask(ctx context.Context, id uint) error {
	deploymentTask, err := s.deploymentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 如果正在运行，先取消
	if deploymentTask.IsRunning() {
		if err := s.CancelDeploymentTask(ctx, id, "删除任务"); err != nil {
			return fmt.Errorf("取消运行中的部署任务失败: %w", err)
		}
	}

	return s.deploymentRepo.Delete(ctx, id)
}

// StartDeploymentTask 开始执行部署任务
func (s *deploymentTaskService) StartDeploymentTask(ctx context.Context, id uint) error {
	log.Printf("[DeploymentTaskService] Starting deployment task: %d", id)

	deploymentTask, err := s.deploymentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if deploymentTask.Status != models.DeploymentTaskStatusPending && deploymentTask.Status != models.DeploymentTaskStatusFailed {
		return fmt.Errorf("只有待执行状态或失败状态的任务才能启动")
	}

	// 创建执行上下文
	execCtx, cancel := context.WithCancel(context.Background())
	execution := &deploymentExecution{
		deploymentTask: deploymentTask,
		ctx:            execCtx,
		cancel:         cancel,
		done:           make(chan struct{}),
	}

	// 注册运行中的任务
	s.mu.Lock()
	s.runningTasks[id] = execution
	s.mu.Unlock()

	// 更新状态为运行中
	deploymentTask.Start()
	if err := s.deploymentRepo.Update(ctx, deploymentTask); err != nil {
		return err
	}

	// 异步执行部署
	go s.executeDeploymentTask(execution)

	log.Printf("[DeploymentTaskService] Deployment task %d started", id)
	return nil
}

// CancelDeploymentTask 取消部署任务
func (s *deploymentTaskService) CancelDeploymentTask(ctx context.Context, id uint, reason string) error {
	log.Printf("[DeploymentTaskService] Cancelling deployment task: %d, reason: %s", id, reason)

	s.mu.Lock()
	execution, exists := s.runningTasks[id]
	if exists {
		execution.cancel() // 取消执行上下文
		delete(s.runningTasks, id)
	}
	s.mu.Unlock()

	// 更新数据库状态
	deploymentTask, err := s.deploymentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	deploymentTask.Cancel(reason)
	deploymentTask.AddLog("INFO", fmt.Sprintf("部署任务已取消: %s", reason), nil, nil, nil)

	return s.deploymentRepo.Update(ctx, deploymentTask)
}

// GetDeploymentTaskLogs 获取部署任务日志
func (s *deploymentTaskService) GetDeploymentTaskLogs(ctx context.Context, id uint, limit int) ([]models.DeploymentLog, error) {
	return s.deploymentRepo.GetLogs(ctx, id, limit)
}

// GetDeploymentTaskResults 获取部署任务结果
func (s *deploymentTaskService) GetDeploymentTaskResults(ctx context.Context, id uint) ([]models.DeploymentResult, error) {
	return s.deploymentRepo.GetResults(ctx, id)
}

// GetDeploymentTaskProgress 获取部署任务进度
func (s *deploymentTaskService) GetDeploymentTaskProgress(ctx context.Context, id uint) (*DeploymentProgress, error) {
	deploymentTask, err := s.deploymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	progress := &DeploymentProgress{
		DeploymentID:   deploymentTask.ID,
		TotalTasks:     deploymentTask.TotalTasks,
		CompletedTasks: deploymentTask.CompletedTasks,
		FailedTasks:    deploymentTask.FailedTasks,
		SkippedTasks:   deploymentTask.SkippedTasks,
		Progress:       deploymentTask.Progress,
		Message:        deploymentTask.Message,
	}

	if deploymentTask.EstimatedTime != nil {
		progress.EstimatedTime = deploymentTask.EstimatedTime.String()
	}

	return progress, nil
}

// BatchCreateDeployment 批量创建部署
func (s *deploymentTaskService) BatchCreateDeployment(ctx context.Context, taskIDs []uint, boxIDs []uint, config *models.DeploymentConfig) (*models.DeploymentTask, error) {
	req := &CreateDeploymentTaskRequest{
		Name:             fmt.Sprintf("批量部署_%d", time.Now().Unix()),
		Description:      fmt.Sprintf("批量部署 %d 个任务到 %d 个盒子", len(taskIDs), len(boxIDs)),
		TaskIDs:          taskIDs,
		TargetBoxIDs:     boxIDs,
		Priority:         3,
		DeploymentConfig: config,
		CreatedBy:        1, // TODO: 从上下文获取用户ID
	}

	return s.CreateDeploymentTask(ctx, req)
}

// GetDeploymentStatistics 获取部署统计
func (s *deploymentTaskService) GetDeploymentStatistics(ctx context.Context) (map[string]interface{}, error) {
	return s.deploymentRepo.GetStatistics(ctx)
}

// CleanupOldTasks 清理旧任务
func (s *deploymentTaskService) CleanupOldTasks(ctx context.Context, olderThan time.Time) (int64, error) {
	return s.deploymentRepo.CleanupCompletedTasks(ctx, olderThan)
}

// executeDeploymentTask 执行部署任务
func (s *deploymentTaskService) executeDeploymentTask(execution *deploymentExecution) {
	defer close(execution.done)

	deploymentTask := execution.deploymentTask
	ctx := execution.ctx

	log.Printf("[DeploymentTaskService] Executing deployment task: %d", deploymentTask.ID)

	// 添加部署开始的详细日志
	deploymentTask.AddDetailedLog("INFO", "start", "deployment_begin",
		fmt.Sprintf("开始执行部署任务 - %s", deploymentTask.Name),
		nil, nil, &[]bool{true}[0], nil, nil, map[string]interface{}{
			"deployment_task_id":    deploymentTask.ID,
			"deployment_task_name":  deploymentTask.Name,
			"total_tasks":           len(deploymentTask.TaskIDs),
			"total_boxes":           len(deploymentTask.TargetBoxIDs),
			"task_ids":              deploymentTask.TaskIDs,
			"target_box_ids":        deploymentTask.TargetBoxIDs,
			"stop_on_first_failure": deploymentTask.DeploymentConfig.StopOnFirstFailure,
			"timeout_seconds":       deploymentTask.DeploymentConfig.TimeoutSeconds,
		})

	taskIDs := []uint(deploymentTask.TaskIDs)
	boxIDs := []uint(deploymentTask.TargetBoxIDs)

	// 并发控制
	semaphore := make(chan struct{}, deploymentTask.DeploymentConfig.ConcurrentLimit)
	var wg sync.WaitGroup

	// 为每个任务和盒子组合创建部署任务
	for _, taskID := range taskIDs {
		for _, boxID := range boxIDs {
			select {
			case <-ctx.Done():
				log.Printf("[DeploymentTaskService] Deployment task %d cancelled", deploymentTask.ID)
				return
			default:
			}

			wg.Add(1)
			go func(tID, bID uint) {
				defer wg.Done()

				// 获取信号量
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				s.deployTask(ctx, deploymentTask, tID, bID)
			}(taskID, boxID)
		}
	}

	// 等待所有部署完成
	wg.Wait()

	// 更新最终状态
	s.finalizeDeploymentTask(deploymentTask)

	// 从运行中任务列表移除
	s.mu.Lock()
	delete(s.runningTasks, deploymentTask.ID)
	s.mu.Unlock()

	log.Printf("[DeploymentTaskService] Deployment task %d completed", deploymentTask.ID)
}

// deployTask 部署单个任务到单个盒子
func (s *deploymentTaskService) deployTask(ctx context.Context, deploymentTask *models.DeploymentTask, taskID, boxID uint) {
	startTime := time.Now()

	deploymentTask.AddLog("INFO", fmt.Sprintf("开始部署任务 %d 到盒子 %d", taskID, boxID), &taskID, &boxID, nil)

	// 执行部署
	resp, err := s.taskDeploymentSvc.DeployTaskWithLogging(ctx, taskID, boxID, deploymentTask)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// 创建部署结果
	result := models.DeploymentResult{
		TaskID:    taskID,
		BoxID:     boxID,
		Success:   err == nil && resp != nil && resp.Success,
		StartTime: startTime,
		EndTime:   &endTime,
		Duration:  duration,
	}

	if err != nil {
		result.Message = err.Error()
		result.Status = string(models.TaskStatusFailed)

		// 获取任务名称用于更详细的日志
		taskName := "Unknown"
		if task, taskErr := s.taskRepo.GetByID(ctx, taskID); taskErr == nil {
			taskName = task.Name
		}

		// 添加详细的失败日志
		success := false
		deploymentTask.AddDetailedLog("ERROR", "deploy", "single_task_failed",
			fmt.Sprintf("单个任务部署失败 - Task: %s (ID: %d), Box: %d", taskName, taskID, boxID),
			&taskID, &boxID, &success, &duration, err, map[string]interface{}{
				"task_id":    taskID,
				"task_name":  taskName,
				"box_id":     boxID,
				"error_type": "DEPLOYMENT_ERROR",
				"duration":   duration.String(),
			})
	} else if resp != nil {
		result.Status = string(resp.Status)
		result.Message = resp.Message
		result.ErrorCode = resp.ErrorCode

		if resp.Success {
			// 获取任务名称用于更详细的日志
			taskName := "Unknown"
			if task, taskErr := s.taskRepo.GetByID(ctx, taskID); taskErr == nil {
				taskName = task.Name
			}

			success := true
			deploymentTask.AddDetailedLog("INFO", "deploy", "single_task_success",
				fmt.Sprintf("单个任务部署成功 - Task: %s (ID: %d), Box: %d", taskName, taskID, boxID),
				&taskID, &boxID, &success, &duration, nil, map[string]interface{}{
					"task_id":    taskID,
					"task_name":  taskName,
					"box_id":     boxID,
					"duration":   duration.String(),
					"error_code": resp.ErrorCode,
				})
		} else {
			// 即使API调用成功，但部署结果显示失败
			taskName := "Unknown"
			if task, taskErr := s.taskRepo.GetByID(ctx, taskID); taskErr == nil {
				taskName = task.Name
			}

			success := false
			deploymentTask.AddDetailedLog("ERROR", "deploy", "single_task_failed",
				fmt.Sprintf("单个任务部署返回失败 - Task: %s (ID: %d), Box: %d", taskName, taskID, boxID),
				&taskID, &boxID, &success, &duration, nil, map[string]interface{}{
					"task_id":    taskID,
					"task_name":  taskName,
					"box_id":     boxID,
					"error_code": resp.ErrorCode,
					"message":    resp.Message,
					"duration":   duration.String(),
				})
		}
	}

	// 添加结果到部署任务
	s.deploymentRepo.AddResult(context.Background(), deploymentTask.ID, result)

	// 检查是否需要在第一个失败时停止
	if !result.Success && deploymentTask.DeploymentConfig.StopOnFirstFailure {
		deploymentTask.AddLog("WARN", "遇到第一个失败，停止后续部署", &taskID, &boxID, nil)
		// TODO: 实现停止逻辑
	}
}

// finalizeDeploymentTask 完成部署任务
func (s *deploymentTaskService) finalizeDeploymentTask(deploymentTask *models.DeploymentTask) {
	ctx := context.Background()

	// 重新加载最新状态
	if latest, err := s.deploymentRepo.GetByID(ctx, deploymentTask.ID); err == nil {
		deploymentTask = latest
	}

	// 生成详细的执行摘要
	summary := s.generateExecutionSummary(ctx, deploymentTask)

	// 根据结果确定最终状态
	if deploymentTask.FailedTasks == 0 {
		deploymentTask.Complete()
		duration := deploymentTask.GetDuration()
		success := true
		deploymentTask.AddDetailedLog("INFO", "completion", "finalize",
			"部署任务全部完成",
			nil, nil, &success, &duration, nil,
			map[string]interface{}{
				"total_tasks":      deploymentTask.TotalTasks,
				"successful_tasks": deploymentTask.CompletedTasks,
				"failed_tasks":     deploymentTask.FailedTasks,
				"success_rate":     deploymentTask.GetSuccessRate(),
				"duration":         deploymentTask.GetDuration().String(),
				"summary":          summary,
			})
	} else {
		// 构建详细的失败信息
		failureDetails := s.getFailureDetails(ctx, deploymentTask)
		errorMessage := fmt.Sprintf("部署完成，但有 %d/%d 个任务失败",
			deploymentTask.FailedTasks, deploymentTask.TotalTasks)

		deploymentTask.Fail(errorMessage)
		duration := deploymentTask.GetDuration()
		success := false
		deploymentTask.AddDetailedLog("ERROR", "completion", "finalize",
			errorMessage,
			nil, nil, &success, &duration, nil,
			map[string]interface{}{
				"total_tasks":      deploymentTask.TotalTasks,
				"successful_tasks": deploymentTask.CompletedTasks,
				"failed_tasks":     deploymentTask.FailedTasks,
				"success_rate":     deploymentTask.GetSuccessRate(),
				"duration":         deploymentTask.GetDuration().String(),
				"summary":          summary,
				"failure_details":  failureDetails,
			})
	}

	// 保存最终状态
	s.deploymentRepo.Update(ctx, deploymentTask)
}

// generateExecutionSummary 生成执行摘要
func (s *deploymentTaskService) generateExecutionSummary(ctx context.Context, deploymentTask *models.DeploymentTask) map[string]interface{} {
	summary := map[string]interface{}{
		"deployment_task_id":   deploymentTask.ID,
		"deployment_task_name": deploymentTask.Name,
		"start_time":           deploymentTask.StartedAt,
		"end_time":             deploymentTask.CompletedAt,
		"duration":             deploymentTask.GetDuration().String(),
		"total_tasks":          deploymentTask.TotalTasks,
		"successful_tasks":     deploymentTask.CompletedTasks,
		"failed_tasks":         deploymentTask.FailedTasks,
		"success_rate":         deploymentTask.GetSuccessRate(),
	}

	// 添加涉及的任务和盒子信息
	if len(deploymentTask.TaskIDs) > 0 {
		summary["task_ids"] = deploymentTask.TaskIDs
	}
	if len(deploymentTask.TargetBoxIDs) > 0 {
		summary["target_box_ids"] = deploymentTask.TargetBoxIDs
	}

	// 添加配置信息
	if deploymentTask.DeploymentConfig.StopOnFirstFailure {
		summary["stop_on_first_failure"] = true
	}
	if deploymentTask.DeploymentConfig.TimeoutSeconds > 0 {
		summary["timeout_seconds"] = deploymentTask.DeploymentConfig.TimeoutSeconds
	}

	return summary
}

// getFailureDetails 获取失败详情
func (s *deploymentTaskService) getFailureDetails(ctx context.Context, deploymentTask *models.DeploymentTask) []map[string]interface{} {
	failureDetails := []map[string]interface{}{}

	// 从部署结果中分析失败原因
	for _, result := range deploymentTask.Results {
		if !result.Success {
			detail := map[string]interface{}{
				"task_id":    result.TaskID,
				"box_id":     result.BoxID,
				"status":     result.Status,
				"message":    result.Message,
				"error_code": result.ErrorCode,
				"start_time": result.StartTime,
				"duration":   result.Duration.String(),
			}

			if result.EndTime != nil {
				detail["end_time"] = *result.EndTime
			}

			failureDetails = append(failureDetails, detail)
		}
	}

	return failureDetails
}

// validateTasksAndBoxes 验证任务和盒子ID
func (s *deploymentTaskService) validateTasksAndBoxes(ctx context.Context, taskIDs, boxIDs []uint) error {
	// 验证任务ID
	for _, taskID := range taskIDs {
		if _, err := s.taskRepo.GetByID(ctx, taskID); err != nil {
			return fmt.Errorf("任务 %d 不存在: %w", taskID, err)
		}
	}

	// TODO: 验证盒子ID
	// for _, boxID := range boxIDs {
	//     if _, err := s.boxRepo.GetByID(ctx, boxID); err != nil {
	//         return fmt.Errorf("盒子 %d 不存在: %w", boxID, err)
	//     }
	// }

	return nil
}

// recoverRunningTasks 恢复运行中的任务
func (s *deploymentTaskService) recoverRunningTasks() {
	ctx := context.Background()

	// 查找所有运行中的部署任务
	runningTasks, err := s.deploymentRepo.FindRunningTasks(ctx)
	if err != nil {
		log.Printf("[DeploymentTaskService] Failed to recover running tasks: %v", err)
		return
	}

	for _, task := range runningTasks {
		log.Printf("[DeploymentTaskService] Recovering deployment task: %d", task.ID)

		// 根据实际情况决定是恢复执行还是标记为失败
		// 这里简化处理，将其标记为失败
		task.Fail("服务重启，任务中断")
		s.deploymentRepo.Update(ctx, task)
	}

	log.Printf("[DeploymentTaskService] Recovered %d running tasks", len(runningTasks))
}
