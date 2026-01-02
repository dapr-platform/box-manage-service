package service

import (
	"box-manage-service/client"
	"box-manage-service/config"
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"
)

// TaskDeploymentStatus 任务部署状态
type TaskDeploymentStatus string

const (
	TaskDeploymentStatusSuccess TaskDeploymentStatus = "success"
	TaskDeploymentStatusFailed  TaskDeploymentStatus = "failed"
	TaskDeploymentStatusPending TaskDeploymentStatus = "pending"
)

// DeploymentResponse 任务部署响应
type DeploymentResponse struct {
	TaskID      uint                 `json:"task_id"`
	BoxID       uint                 `json:"box_id"`
	Success     bool                 `json:"success"`
	Status      TaskDeploymentStatus `json:"status"`
	ExecutionID string               `json:"execution_id"`
	Message     string               `json:"message,omitempty"`
	ErrorCode   string               `json:"error_code,omitempty"`
}

// TaskStatistics 任务统计信息
type TaskStatistics struct {
	FPS             float64 `json:"fps"`
	AverageLatency  float64 `json:"average_latency"`
	ProcessedFrames int64   `json:"processed_frames"`
	TotalFrames     int64   `json:"total_frames"`
	InferenceCount  int64   `json:"inference_count"`
	ForwardSuccess  int64   `json:"forward_success"`
	ForwardFailed   int64   `json:"forward_failed"`
}

// BoxTaskStatus 盒子任务状态
type BoxTaskStatus struct {
	TaskID      uint            `json:"task_id"`
	Status      string          `json:"status"`
	Progress    float64         `json:"progress"`
	Error       string          `json:"error,omitempty"`
	Message     string          `json:"message,omitempty"`
	LastUpdated string          `json:"last_updated,omitempty"`
	Statistics  *TaskStatistics `json:"statistics,omitempty"`
}

type taskDeploymentService struct {
	taskRepo           repository.TaskRepository
	boxRepo            repository.BoxRepository
	videoSourceRepo    repository.VideoSourceRepository
	originalModelRepo  repository.OriginalModelRepository
	convertedModelRepo repository.ConvertedModelRepository
	config             config.VideoConfig
	logService         SystemLogService // 系统日志服务
	sseService         SSEService       // SSE服务
}

func NewTaskDeploymentService(
	taskRepo repository.TaskRepository,
	boxRepo repository.BoxRepository,
	videoSourceRepo repository.VideoSourceRepository,
	originalModelRepo repository.OriginalModelRepository,
	convertedModelRepo repository.ConvertedModelRepository,
	cfg config.VideoConfig,
	logService SystemLogService,
	sseService SSEService,
) TaskDeploymentService {
	return &taskDeploymentService{
		taskRepo:           taskRepo,
		boxRepo:            boxRepo,
		videoSourceRepo:    videoSourceRepo,
		originalModelRepo:  originalModelRepo,
		convertedModelRepo: convertedModelRepo,
		config:             cfg,
		logService:         logService,
		sseService:         sseService,
	}
}

func (s *taskDeploymentService) DeployTask(ctx context.Context, taskID uint, boxID uint) (*DeploymentResponse, error) {
	return s.DeployTaskWithLogging(ctx, taskID, boxID, nil)
}

func (s *taskDeploymentService) DeployTaskWithLogging(ctx context.Context, taskID uint, boxID uint, deploymentTask *models.DeploymentTask) (*DeploymentResponse, error) {
	log.Printf("[TaskDeploymentService] DeployTask started - TaskID: %d, BoxID: %d", taskID, boxID)

	// 记录任务部署开始日志
	if s.logService != nil {
		s.logService.Info("task_deployment_service", "任务部署开始",
			fmt.Sprintf("开始部署任务 %d 到盒子 %d", taskID, boxID),
			WithMetadata(map[string]interface{}{
				"task_id": taskID,
				"box_id":  boxID,
			}))
	}

	// 获取任务信息
	log.Printf("[TaskDeploymentService] Step 1/7: Retrieving task information - TaskID: %d", taskID)
	if deploymentTask != nil {
		deploymentTask.AddDetailedLog("INFO", "init", "get_task",
			fmt.Sprintf("开始获取任务信息 - TaskID: %d", taskID),
			&taskID, &boxID, nil, nil, nil, nil)
	}

	startTime := time.Now()
	task, err := s.taskRepo.GetByID(ctx, taskID)
	duration := time.Since(startTime)

	if err != nil {
		log.Printf("[TaskDeploymentService] Step 1/7: Failed to retrieve task - TaskID: %d, Error: %v", taskID, err)

		// 记录获取任务失败日志
		if s.logService != nil {
			s.logService.Error("task_deployment_service", "获取任务失败",
				fmt.Sprintf("无法获取任务 %d 的信息", taskID), err,
				WithMetadata(map[string]interface{}{
					"task_id":     taskID,
					"box_id":      boxID,
					"error_code":  "TASK_NOT_FOUND",
					"duration_ms": duration.Milliseconds(),
				}))
		}

		if deploymentTask != nil {
			success := false
			deploymentTask.AddDetailedLog("ERROR", "init", "get_task",
				fmt.Sprintf("获取任务失败 - TaskID: %d", taskID),
				&taskID, &boxID, &success, &duration, err, map[string]interface{}{
					"error_code": "TASK_NOT_FOUND",
				})
		}
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("获取任务失败: %v", err),
			ErrorCode: "TASK_NOT_FOUND",
		}, err
	}

	log.Printf("[TaskDeploymentService] Step 1/7: Successfully retrieved task - TaskID: %s, Name: %s, VideoSourceID: %d",
		task.TaskID, task.Name, task.VideoSourceID)
	if deploymentTask != nil {
		success := true
		deploymentTask.AddDetailedLog("INFO", "init", "get_task",
			fmt.Sprintf("成功获取任务信息 - TaskID: %s, Name: %s", task.TaskID, task.Name),
			&taskID, &boxID, &success, &duration, nil, map[string]interface{}{
				"task_id":         task.TaskID,
				"task_name":       task.Name,
				"video_source_id": task.VideoSourceID,
			})
	}

	// 获取盒子信息
	log.Printf("[TaskDeploymentService] Step 2/7: Retrieving box information - BoxID: %d", boxID)
	if deploymentTask != nil {
		deploymentTask.AddDetailedLog("INFO", "validate", "get_box",
			fmt.Sprintf("开始获取盒子信息 - BoxID: %d", boxID),
			&taskID, &boxID, nil, nil, nil, nil)
	}

	startTime = time.Now()
	box, err := s.boxRepo.GetByID(ctx, boxID)
	duration = time.Since(startTime)

	if err != nil {
		log.Printf("[TaskDeploymentService] Step 2/7: Failed to retrieve box - BoxID: %d, Error: %v", boxID, err)
		if deploymentTask != nil {
			success := false
			deploymentTask.AddDetailedLog("ERROR", "validate", "get_box",
				fmt.Sprintf("获取盒子失败 - BoxID: %d", boxID),
				&taskID, &boxID, &success, &duration, err, map[string]interface{}{
					"error_code": "BOX_NOT_FOUND",
				})
		}
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("获取盒子失败: %v", err),
			ErrorCode: "BOX_NOT_FOUND",
		}, err
	}

	log.Printf("[TaskDeploymentService] Step 2/7: Successfully retrieved box - BoxID: %d, Name: %s, IPAddress: %s, Status: %s",
		box.ID, box.Name, box.IPAddress, box.Status)
	if deploymentTask != nil {
		success := true
		deploymentTask.AddDetailedLog("INFO", "validate", "get_box",
			fmt.Sprintf("成功获取盒子信息 - BoxID: %d, Name: %s", box.ID, box.Name),
			&taskID, &boxID, &success, &duration, nil, map[string]interface{}{
				"box_id":     box.ID,
				"box_name":   box.Name,
				"box_ip":     box.IPAddress,
				"box_status": box.Status,
				"box_port":   box.Port,
			})
	}

	// 检查盒子状态
	log.Printf("[TaskDeploymentService] Step 3/7: Checking box status - BoxID: %d, Status: %s", boxID, box.Status)
	if box.Status != models.BoxStatusOnline {
		log.Printf("[TaskDeploymentService] Step 3/7: Box is not online - BoxID: %d, Status: %s", boxID, box.Status)
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("盒子不在线，当前状态: %s", box.Status),
			ErrorCode: "BOX_OFFLINE",
		}, fmt.Errorf("盒子不在线")
	}
	log.Printf("[TaskDeploymentService] Step 3/7: Box status check passed - BoxID: %d is online", boxID)

	// 创建盒子客户端并健康检查
	log.Printf("[TaskDeploymentService] Step 4/7: Creating box client and performing health check - BoxID: %d, Address: %s:%d",
		boxID, box.IPAddress, box.Port)
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))

	if err := boxClient.Health(ctx); err != nil {
		log.Printf("[TaskDeploymentService] Step 4/7: Box health check failed - BoxID: %d, Error: %v", boxID, err)
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("盒子健康检查失败: %v", err),
			ErrorCode: "BOX_HEALTH_CHECK_FAILED",
		}, err
	}
	log.Printf("[TaskDeploymentService] Step 4/7: Box health check passed - BoxID: %d is healthy", boxID)

	// 转换任务配置
	log.Printf("[TaskDeploymentService] Step 5/7: Converting task configuration for box - TaskID: %d, BoxID: %d", taskID, boxID)
	if deploymentTask != nil {
		deploymentTask.AddDetailedLog("INFO", "prepare", "convert_config",
			fmt.Sprintf("开始转换任务配置 - TaskID: %d, BoxID: %d", taskID, boxID),
			&taskID, &boxID, nil, nil, nil, map[string]interface{}{
				"inference_tasks_count": len(task.InferenceTasks),
				"rois_count":            len(task.ROIs),
			})
	}

	startTime = time.Now()
	boxTask, err := s.convertToBoxTask(ctx, task, box)
	duration = time.Since(startTime)

	if err != nil {
		log.Printf("[TaskDeploymentService] Step 5/7: Failed to convert task configuration - TaskID: %d, BoxID: %d, Error: %v",
			taskID, boxID, err)
		if deploymentTask != nil {
			success := false
			deploymentTask.AddDetailedLog("ERROR", "prepare", "convert_config",
				fmt.Sprintf("转换任务配置失败 - TaskID: %d, BoxID: %d", taskID, boxID),
				&taskID, &boxID, &success, &duration, err, map[string]interface{}{
					"error_code": "TASK_CONVERSION_FAILED",
				})
		}
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("转换任务配置失败: %v", err),
			ErrorCode: "TASK_CONVERSION_FAILED",
		}, err
	}

	jsonData, _ := json.MarshalIndent(boxTask, "", "  ")
	log.Printf("[TaskDeploymentService] Step 5/7: Task configuration converted successfully")
	log.Printf("[TaskDeploymentService] BoxTask JSON:\n%s", string(jsonData))
	if deploymentTask != nil {
		success := true
		deploymentTask.AddDetailedLog("INFO", "prepare", "convert_config",
			fmt.Sprintf("任务配置转换成功 - TaskID: %d, BoxID: %d", taskID, boxID),
			&taskID, &boxID, &success, &duration, nil, map[string]interface{}{
				"box_task_id":           boxTask.TaskID,
				"rtsp_url":              boxTask.RTSPUrl,
				"inference_tasks_count": len(boxTask.InferenceTasks),
				"rois_count":            len(boxTask.ROIs),
				"auto_start":            boxTask.AutoStart,
			})
	}

	// Step 6: 检查盒子上是否已存在该任务，决定创建还是更新
	log.Printf("[TaskDeploymentService] Step 6/7: Checking if task exists on box - TaskID: %d, BoxID: %d, BoxTaskID: %s",
		taskID, boxID, boxTask.TaskID)

	// 先查询盒子上是否已有该任务
	checkCtx, checkCancel := context.WithTimeout(ctx, 10*time.Second)
	existingTask, checkErr := boxClient.GetTask(checkCtx, boxTask.TaskID)
	checkCancel()

	taskExistsOnBox := checkErr == nil && existingTask != nil
	var operationType string // "create" 或 "update"

	if taskExistsOnBox {
		operationType = "update"
		log.Printf("[TaskDeploymentService] Step 6/7: Task already exists on box, will update - TaskID: %s", boxTask.TaskID)
		if deploymentTask != nil {
			deploymentTask.AddDetailedLog("INFO", "deploy", "check_task",
				fmt.Sprintf("任务已存在于盒子上，将执行更新 - TaskID: %d, BoxID: %d, BoxTaskID: %s", taskID, boxID, boxTask.TaskID),
				&taskID, &boxID, nil, nil, nil, map[string]interface{}{
					"box_task_id":    boxTask.TaskID,
					"operation_type": "update",
				})
		}
	} else {
		operationType = "create"
		log.Printf("[TaskDeploymentService] Step 6/7: Task does not exist on box, will create - TaskID: %s", boxTask.TaskID)
		if deploymentTask != nil {
			deploymentTask.AddDetailedLog("INFO", "deploy", "check_task",
				fmt.Sprintf("任务不存在于盒子上，将执行创建 - TaskID: %d, BoxID: %d, BoxTaskID: %s", taskID, boxID, boxTask.TaskID),
				&taskID, &boxID, nil, nil, nil, map[string]interface{}{
					"box_task_id":    boxTask.TaskID,
					"operation_type": "create",
				})
		}
	}

	// 执行创建或更新操作
	startTime = time.Now()
	if taskExistsOnBox {
		// 更新已存在的任务
		log.Printf("[TaskDeploymentService] Step 6/7: Updating existing task on box - TaskID: %s", boxTask.TaskID)
		err = boxClient.UpdateTask(ctx, boxTask.TaskID, boxTask)
	} else {
		// 创建新任务
		log.Printf("[TaskDeploymentService] Step 6/7: Creating new task on box - TaskID: %s", boxTask.TaskID)
		err = boxClient.CreateTask(ctx, boxTask)
	}
	duration = time.Since(startTime)

	if err != nil {
		log.Printf("[TaskDeploymentService] Step 6/7: Failed to %s task on box - TaskID: %d, BoxID: %d, Error: %v",
			operationType, taskID, boxID, err)
		if deploymentTask != nil {
			success := false
			deploymentTask.AddDetailedLog("ERROR", "deploy", operationType+"_task",
				fmt.Sprintf("向盒子%s任务失败 - TaskID: %d, BoxID: %d", operationType, taskID, boxID),
				&taskID, &boxID, &success, &duration, err, map[string]interface{}{
					"error_code":     "BOX_DEPLOYMENT_FAILED",
					"box_task_id":    boxTask.TaskID,
					"operation_type": operationType,
				})
		}
		// 发送任务部署失败事件
		if s.sseService != nil {
			metadata := map[string]interface{}{
				"task_id":        taskID,
				"box_id":         boxID,
				"error":          err.Error(),
				"error_code":     "BOX_DEPLOYMENT_FAILED",
				"operation_type": operationType,
				"source":         "task_deployment_service",
				"source_id":      fmt.Sprintf("deployment_%d_%d", taskID, boxID),
				"title":          fmt.Sprintf("任务%s失败", operationType),
				"message":        fmt.Sprintf("任务 %d %s到盒子 %d 失败: %v", taskID, operationType, boxID, err),
			}
			s.sseService.BroadcastDeploymentTaskFailed(taskID, err.Error(), metadata)
		}

		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("%s任务到盒子失败: %v", operationType, err),
			ErrorCode: "BOX_DEPLOYMENT_FAILED",
		}, err
	}

	log.Printf("[TaskDeploymentService] Step 6/7: Task %s on box successfully - TaskID: %d, BoxID: %d, Duration: %v",
		operationType, taskID, boxID, duration)

	// 验证任务是否真正创建/更新成功 - 尝试从盒子获取任务状态
	log.Printf("[TaskDeploymentService] Step 6.5/7: Verifying task %s on box - TaskID: %s", operationType, boxTask.TaskID)
	verifyCtx, verifyCancel := context.WithTimeout(ctx, 10*time.Second)
	defer verifyCancel()
	taskOnBox, verifyErr := boxClient.GetTask(verifyCtx, boxTask.TaskID)
	if verifyErr != nil {
		log.Printf("[TaskDeploymentService] Step 6.5/7: WARNING - Failed to verify task on box - TaskID: %s, Error: %v",
			boxTask.TaskID, verifyErr)
		// 记录警告但不阻断流程
		if s.logService != nil {
			s.logService.Warn("task_deployment_service", "任务验证警告",
				fmt.Sprintf("无法验证盒子上的任务 %s: %v", boxTask.TaskID, verifyErr))
		}
	} else if taskOnBox != nil {
		log.Printf("[TaskDeploymentService] Step 6.5/7: Task verified on box - TaskID: %s, DevID: %s, RTSPUrl: %s",
			taskOnBox.TaskID, taskOnBox.DevID, taskOnBox.RTSPUrl)
	} else {
		log.Printf("[TaskDeploymentService] Step 6.5/7: WARNING - Task not found on box after %s - TaskID: %s",
			operationType, boxTask.TaskID)
		if s.logService != nil {
			s.logService.Warn("task_deployment_service", "任务验证警告",
				fmt.Sprintf("任务 %s %s后在盒子上未找到", boxTask.TaskID, operationType))
		}
	}

	if deploymentTask != nil {
		success := true
		deploymentTask.AddDetailedLog("INFO", "deploy", operationType+"_task",
			fmt.Sprintf("成功向盒子%s任务 - TaskID: %d, BoxID: %d", operationType, taskID, boxID),
			&taskID, &boxID, &success, &duration, nil, map[string]interface{}{
				"box_task_id":            boxTask.TaskID,
				"deployment_duration_ms": duration.Milliseconds(),
				"operation_type":         operationType,
			})
	}

	// 更新任务状态
	log.Printf("[TaskDeploymentService] Step 7/7: Updating task status in database - TaskID: %d, BoxID: %d", taskID, boxID)
	task.AssignToBox(boxID) // 使用新方法更新调度状态
	if err := s.taskRepo.Update(ctx, task); err != nil {
		log.Printf("[TaskDeploymentService] Step 7/7: Failed to update task status, rolling back - TaskID: %d, BoxID: %d, Error: %v",
			taskID, boxID, err)
		// 回滚：删除盒子上的任务
		if deleteErr := boxClient.DeleteTask(ctx, task.TaskID); deleteErr != nil {
			log.Printf("[TaskDeploymentService] Rollback failed: could not delete task from box - TaskID: %d, BoxID: %d, Error: %v",
				taskID, boxID, deleteErr)
		} else {
			log.Printf("[TaskDeploymentService] Rollback successful: task deleted from box - TaskID: %d, BoxID: %d", taskID, boxID)
		}
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("更新任务状态失败: %v", err),
			ErrorCode: "TASK_UPDATE_FAILED",
		}, err
	}
	log.Printf("[TaskDeploymentService] Step 7/7: Task status updated successfully - TaskID: %d, BoxID: %d, Status: %s",
		taskID, boxID, task.Status)

	// 生成执行ID
	executionID := fmt.Sprintf("exec_%d_%d_%d", taskID, boxID, time.Now().Unix())

	// 注意：SSE 消息由调用方（如 deployment_task_service）在更新部署任务状态后发送
	// 这里不再发送 SSE 消息，避免在部署任务统计信息更新前发送通知

	log.Printf("[TaskDeploymentService] Task deployment completed successfully - TaskID: %d, BoxID: %d, ExecutionID: %s",
		taskID, boxID, executionID)
	return &DeploymentResponse{
		TaskID:      taskID,
		BoxID:       boxID,
		Success:     true,
		Status:      TaskDeploymentStatusSuccess,
		ExecutionID: executionID,
		Message:     "任务下发成功",
	}, nil
}

func (s *taskDeploymentService) UndeployTask(ctx context.Context, taskID uint, boxID uint) error {
	log.Printf("[TaskDeploymentService] UndeployTask started - TaskID: %d, BoxID: %d", taskID, boxID)

	log.Printf("[TaskDeploymentService] Getting task information - TaskID: %d", taskID)
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		log.Printf("[TaskDeploymentService] Failed to get task - TaskID: %d, Error: %v", taskID, err)
		return fmt.Errorf("获取任务失败: %w", err)
	}
	log.Printf("[TaskDeploymentService] Task retrieved successfully - TaskID: %s, Name: %s", task.TaskID, task.Name)

	log.Printf("[TaskDeploymentService] Getting box information - BoxID: %d", boxID)
	box, err := s.boxRepo.GetByID(ctx, boxID)
	if err != nil {
		log.Printf("[TaskDeploymentService] Failed to get box - BoxID: %d, Error: %v", boxID, err)
		return fmt.Errorf("获取盒子失败: %w", err)
	}
	log.Printf("[TaskDeploymentService] Box retrieved successfully - BoxID: %d, Name: %s", box.ID, box.Name)

	log.Printf("[TaskDeploymentService] Creating box client and deleting task from box - BoxID: %d, TaskID: %s", boxID, task.TaskID)
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))
	if err := boxClient.DeleteTask(ctx, task.TaskID); err != nil {
		log.Printf("[TaskDeploymentService] Failed to delete task from box - BoxID: %d, TaskID: %s, Error: %v", boxID, task.TaskID, err)
		// 继续执行，更新本地状态
	} else {
		log.Printf("[TaskDeploymentService] Task deleted from box successfully - BoxID: %d, TaskID: %s", boxID, task.TaskID)
	}

	log.Printf("[TaskDeploymentService] Updating task status to pending - TaskID: %d", taskID)
	task.UnassignFromBox() // 使用新方法更新调度状态
	if err := s.taskRepo.Update(ctx, task); err != nil {
		log.Printf("[TaskDeploymentService] Failed to update task status - TaskID: %d, Error: %v", taskID, err)
		return err
	}

	log.Printf("[TaskDeploymentService] UndeployTask completed successfully - TaskID: %d, BoxID: %d", taskID, boxID)
	return nil
}

func (s *taskDeploymentService) RedeployTask(ctx context.Context, taskID uint, oldBoxID *uint, newBoxID uint) (*DeploymentResponse, error) {
	log.Printf("[TaskDeploymentService] RedeployTask started - TaskID: %d, OldBoxID: %v, NewBoxID: %d", taskID, oldBoxID, newBoxID)

	// 如果需要从旧盒子上卸载任务
	if oldBoxID != nil && *oldBoxID != newBoxID {
		log.Printf("[TaskDeploymentService] Task needs to be moved from box %d to box %d - TaskID: %d", *oldBoxID, newBoxID, taskID)
		if err := s.UndeployTask(ctx, taskID, *oldBoxID); err != nil {
			log.Printf("[TaskDeploymentService] Failed to undeploy from old box - TaskID: %d, OldBoxID: %d, Error: %v", taskID, *oldBoxID, err)
			// 继续执行部署到新盒子，可能旧盒子已经不可用
		} else {
			log.Printf("[TaskDeploymentService] Successfully undeployed from old box - TaskID: %d, OldBoxID: %d", taskID, *oldBoxID)
		}
	} else {
		log.Printf("[TaskDeploymentService] Task deployment target unchanged - TaskID: %d, BoxID: %d", taskID, newBoxID)
	}

	log.Printf("[TaskDeploymentService] Deploying task to new box - TaskID: %d, NewBoxID: %d", taskID, newBoxID)
	result, err := s.DeployTask(ctx, taskID, newBoxID)
	if err != nil {
		log.Printf("[TaskDeploymentService] RedeployTask failed - TaskID: %d, NewBoxID: %d, Error: %v", taskID, newBoxID, err)
	} else {
		log.Printf("[TaskDeploymentService] RedeployTask completed successfully - TaskID: %d, NewBoxID: %d", taskID, newBoxID)
	}
	return result, err
}

func (s *taskDeploymentService) convertToBoxTask(ctx context.Context, task *models.Task, box *models.Box) (*client.BoxTaskConfig, error) {
	log.Printf("[TaskDeploymentService] convertToBoxTask started - TaskID: %s, BoxID: %d", task.TaskID, box.ID)

	// 获取视频源信息
	log.Printf("[TaskDeploymentService] Getting video source information - VideoSourceID: %d", task.VideoSourceID)
	videoSource, err := s.videoSourceRepo.GetByID(task.VideoSourceID)
	if err != nil {
		log.Printf("[TaskDeploymentService] Failed to get video source - VideoSourceID: %d, Error: %v", task.VideoSourceID, err)
		return nil, fmt.Errorf("获取视频源失败: %w", err)
	}
	log.Printf("[TaskDeploymentService] Video source retrieved successfully - VideoSourceID: %d, Type: %s, URL: %s",
		videoSource.ID, videoSource.Type, videoSource.URL)

	// 根据视频源类型获取RTSP URL
	log.Printf("[TaskDeploymentService] Converting video source to RTSP URL - VideoSourceID: %d, Type: %s",
		videoSource.ID, videoSource.Type)
	rtspUrl, err := s.getRTSPUrlFromVideoSource(videoSource)
	if err != nil {
		log.Printf("[TaskDeploymentService] Failed to get RTSP URL - VideoSourceID: %d, Error: %v", videoSource.ID, err)
		return nil, fmt.Errorf("获取RTSP URL失败: %w", err)
	}
	log.Printf("[TaskDeploymentService] RTSP URL generated successfully - RTSP: %s", rtspUrl)

	// 转换ROI配置
	log.Printf("[TaskDeploymentService] Converting ROI configurations - ROI count: %d", len(task.ROIs))
	var boxROIs []client.BoxROIConfig
	for i, roi := range task.ROIs {
		log.Printf("[TaskDeploymentService] Converting ROI %d - ID: %d, Name: %s, Width: %d, Height: %d",
			i, roi.ID, roi.Name, roi.Width, roi.Height)
		boxROI := client.BoxROIConfig{
			ID:     int(roi.ID),
			Name:   roi.Name,
			Width:  roi.Width,
			Height: roi.Height,
			X:      roi.X,
			Y:      roi.Y,
			Areas:  make([]client.BoxAreaPoint, len(roi.Areas)),
		}
		for j, area := range roi.Areas {
			boxROI.Areas[j] = client.BoxAreaPoint{X: area.X, Y: area.Y}
		}
		boxROIs = append(boxROIs, boxROI)
	}
	log.Printf("[TaskDeploymentService] ROI configurations converted successfully - Total ROIs: %d", len(boxROIs))

	// 创建盒子客户端用于检查模型
	log.Printf("[TaskDeploymentService] Creating box client for model checking - BoxID: %d, Address: %s:%d",
		box.ID, box.IPAddress, box.Port)
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))

	// 转换推理任务配置
	log.Printf("[TaskDeploymentService] Converting inference tasks - Inference task count: %d", len(task.InferenceTasks))
	var boxInferenceTasks []client.BoxInferenceTask
	for i, inferenceTask := range task.InferenceTasks {
		log.Printf("[TaskDeploymentService] Processing inference task %d - Type: %s, ModelName: %s, OriginalModelID: %s",
			i, inferenceTask.Type, inferenceTask.ModelName, inferenceTask.OriginalModelID)

		// 生成模型key并检查模型可用性
		log.Printf("[TaskDeploymentService] Generating model key for inference task %d", i)
		modelKey, err := s.generateModelKeyForBox(ctx, &inferenceTask, box)
		if err != nil {
			log.Printf("[TaskDeploymentService] Failed to generate model key for inference task %d - Error: %v", i, err)
			return nil, fmt.Errorf("生成模型Key失败: %w", err)
		}
		log.Printf("[TaskDeploymentService] Model key generated for inference task %d - ModelKey: %s", i, modelKey)

		// 检查并确保模型在盒子上可用
		log.Printf("[TaskDeploymentService] Ensuring model availability for inference task %d - ModelKey: %s", i, modelKey)
		if err := s.ensureModelAvailableOnBox(ctx, boxClient, modelKey, &inferenceTask, box); err != nil {
			log.Printf("[TaskDeploymentService] Failed to ensure model availability for inference task %d - ModelKey: %s, Error: %v",
				i, modelKey, err)
			return nil, fmt.Errorf("确保模型 %s 在盒子上可用失败: %w", modelKey, err)
		}
		log.Printf("[TaskDeploymentService] Model availability ensured for inference task %d - ModelKey: %s", i, modelKey)

		// 创建盒子推理任务配置
		log.Printf("[TaskDeploymentService] Creating box inference task config %d", i)
		boxInferenceTask := client.BoxInferenceTask{
			Type:            inferenceTask.Type,
			ModelKey:        modelKey, // 使用modelKey而不是modelName
			Threshold:       inferenceTask.Threshold,
			SendSSEImage:    inferenceTask.SendSSEImage,
			BusinessProcess: inferenceTask.BusinessProcess,
			RTSPPushUrl:     inferenceTask.RtspPushUrl,
			ROIIds:          inferenceTask.ROIIds, // 添加ROI关联
		}

		// 处理转发配置列表
		var forwardInfos []client.BoxForwardInfo

		// 处理新的ForwardInfos数组
		if len(inferenceTask.ForwardInfos) > 0 {
			for j, forwardInfo := range inferenceTask.ForwardInfos {
				log.Printf("[TaskDeploymentService] Adding forward info %d for inference task %d - Type: %s, Host: %s:%d",
					j, i, forwardInfo.Type, forwardInfo.Host, forwardInfo.Port)
				forwardInfos = append(forwardInfos, client.BoxForwardInfo{
					Enabled:  forwardInfo.Enabled,
					Type:     forwardInfo.Type,
					Host:     forwardInfo.Host,
					Port:     forwardInfo.Port,
					Topic:    forwardInfo.Topic,
					Username: forwardInfo.Username,
					Password: forwardInfo.Password,
				})
			}
		}

		// 向后兼容：处理单个ForwardInfo
		if inferenceTask.ForwardInfo != nil && len(forwardInfos) == 0 {
			log.Printf("[TaskDeploymentService] Adding legacy forward info for inference task %d - Type: %s, Host: %s:%d",
				i, inferenceTask.ForwardInfo.Type, inferenceTask.ForwardInfo.Host, inferenceTask.ForwardInfo.Port)
			forwardInfos = append(forwardInfos, client.BoxForwardInfo{
				Enabled:  true, // 默认启用
				Type:     inferenceTask.ForwardInfo.Type,
				Host:     inferenceTask.ForwardInfo.Host,
				Port:     inferenceTask.ForwardInfo.Port,
				Topic:    inferenceTask.ForwardInfo.Topic,
				Username: inferenceTask.ForwardInfo.Username,
				Password: inferenceTask.ForwardInfo.Password,
			})
		}

		boxInferenceTask.ForwardInfos = forwardInfos
		boxInferenceTasks = append(boxInferenceTasks, boxInferenceTask)
		log.Printf("[TaskDeploymentService] Inference task %d converted successfully", i)
	}
	log.Printf("[TaskDeploymentService] All inference tasks converted successfully - Total: %d", len(boxInferenceTasks))

	// 创建最终的盒子任务配置
	devID := fmt.Sprintf("task_%d", task.ID)
	log.Printf("[TaskDeploymentService] Creating final box task configuration - TaskID: %s, DevID: %s, RTSP: %s",
		task.TaskID, devID, rtspUrl)

	// 转换任务级别转发配置
	var taskLevelForwardInfos []client.BoxForwardInfo
	for _, forwardInfo := range task.TaskLevelForwardInfos {
		taskLevelForwardInfos = append(taskLevelForwardInfos, client.BoxForwardInfo{
			Enabled:  forwardInfo.Enabled,
			Type:     forwardInfo.Type,
			Host:     forwardInfo.Host,
			Port:     forwardInfo.Port,
			Topic:    forwardInfo.Topic,
			Username: forwardInfo.Username,
			Password: forwardInfo.Password,
		})
	}

	boxTaskConfig := &client.BoxTaskConfig{
		TaskID:    task.TaskID,
		DevID:     devID, // 使用任务ID生成DevID
		RTSPUrl:   rtspUrl,
		SkipFrame: task.SkipFrame,
		AutoStart: task.AutoStart,
		OutputSettings: client.BoxOutputSettings{
			SendFullImage: task.OutputSettings.SendFullImage,
		},
		ROIs:                  boxROIs,
		InferenceTasks:        boxInferenceTasks,
		UseROItoInference:     task.UseROItoInference, // 添加ROI推理开关
		TaskLevelForwardInfos: taskLevelForwardInfos,  // 任务级别转发配置
	}

	log.Printf("[TaskDeploymentService] convertToBoxTask completed successfully - TaskID: %s, BoxID: %d, DevID: %s",
		task.TaskID, box.ID, devID)
	return boxTaskConfig, nil
}

// getRTSPUrlFromVideoSource 根据视频源类型获取RTSP URL
func (s *taskDeploymentService) getRTSPUrlFromVideoSource(videoSource *models.VideoSource) (string, error) {
	log.Printf("[TaskDeploymentService] Getting RTSP URL for video source - ID: %d, Type: %s", videoSource.ID, videoSource.Type)

	switch videoSource.Type {
	case models.VideoSourceTypeStream:
		// 实时流，直接使用视频源的URL
		log.Printf("[TaskDeploymentService] Stream type - using original URL: %s", videoSource.URL)
		return videoSource.URL, nil

	case models.VideoSourceTypeFile:
		// 文件类型，使用配置的RTSPPrefix + 去掉routePrefix的playUrl
		rtspUrl := s.buildRTSPUrlFromPlayUrl(videoSource.PlayURL)
		log.Printf("[TaskDeploymentService] File type - converted PlayURL %s to RTSP: %s", videoSource.PlayURL, rtspUrl)
		return rtspUrl, nil

	default:
		log.Printf("[TaskDeploymentService] Unsupported video source type: %s", videoSource.Type)
		return "", fmt.Errorf("不支持的视频源类型: %s", videoSource.Type)
	}
}

// buildRTSPUrlFromPlayUrl 从PlayURL构建RTSP URL
func (s *taskDeploymentService) buildRTSPUrlFromPlayUrl(playUrl string) string {
	log.Printf("[TaskDeploymentService] Building RTSP URL from PlayURL: %s", playUrl)

	// 去掉routePrefix，提取流路径
	// 例如：http://localhost:8080/api/v1/video/files/1/play/live/test.flv -> /live/test
	if strings.Contains(playUrl, s.config.PlayURL.RoutePrefix) {
		parts := strings.Split(playUrl, s.config.PlayURL.RoutePrefix)
		if len(parts) >= 2 {
			streamPath := parts[1]
			// 去掉文件扩展名
			if strings.Contains(streamPath, ".") {
				lastDot := strings.LastIndex(streamPath, ".")
				streamPath = streamPath[:lastDot]
			}

			rtspUrl := s.config.ZLMediaKit.RTSPPrefix + streamPath
			log.Printf("[TaskDeploymentService] Extracted stream path: %s, Final RTSP URL: %s", streamPath, rtspUrl)
			return rtspUrl
		}
	}

	// 默认处理
	defaultUrl := s.config.ZLMediaKit.RTSPPrefix + "/live/default"
	log.Printf("[TaskDeploymentService] Using default RTSP URL: %s", defaultUrl)
	return defaultUrl
}

// generateModelKeyForBox 根据OriginalModelID和盒子硬件生成模型Key
func (s *taskDeploymentService) generateModelKeyForBox(ctx context.Context, inferenceTask *models.InferenceTask, box *models.Box) (string, error) {
	log.Printf("[TaskDeploymentService] generateModelKeyForBox started - OriginalModelID: %s", inferenceTask.OriginalModelID)

	// 检查必要的依赖
	if s.originalModelRepo == nil {
		log.Printf("[TaskDeploymentService] originalModelRepo is nil, falling back to ModelName: %s", inferenceTask.ModelName)
		return s.selectModelForBox(ctx, inferenceTask.ModelName, box.Meta.SupportedHardware)
	}

	// 如果没有OriginalModelID，使用传统方式（向后兼容）
	if inferenceTask.OriginalModelID == "" {
		log.Printf("[TaskDeploymentService] No OriginalModelID, falling back to ModelName: %s", inferenceTask.ModelName)
		return s.selectModelForBox(ctx, inferenceTask.ModelName, box.Meta.SupportedHardware)
	}

	// 根据OriginalModelID获取原始模型信息
	originalModelIDUint, err := s.parseOriginalModelID(inferenceTask.OriginalModelID)
	if err != nil {
		return "", fmt.Errorf("解析OriginalModelID失败: %w", err)
	}

	originalModel, err := s.originalModelRepo.GetByID(ctx, originalModelIDUint)
	if err != nil {
		return "", fmt.Errorf("获取原始模型失败: %w", err)
	}
	if originalModel == nil {
		return "", fmt.Errorf("原始模型不存在: %d", originalModelIDUint)
	}

	// 查找适用于盒子的转换后模型
	convertedModel, err := s.findConvertedModelForBox(ctx, originalModel, box)
	if err != nil {
		return "", fmt.Errorf("查找转换后模型失败: %w", err)
	}

	if convertedModel == nil {
		return "", fmt.Errorf("未找到适用于硬件 %v 的转换后模型", box.Meta.SupportedHardware)
	}

	// 直接使用转换后模型的ModelKey
	modelKey := convertedModel.ModelKey

	log.Printf("[TaskDeploymentService] Found converted model key: %s for model: %s (ID: %d)",
		modelKey, originalModel.Name, originalModel.ID)

	return modelKey, nil
}

// parseOriginalModelID 解析OriginalModelID字符串为uint
func (s *taskDeploymentService) parseOriginalModelID(originalModelID string) (uint, error) {
	if originalModelID == "" {
		return 0, fmt.Errorf("OriginalModelID为空")
	}

	// 将字符串转换为uint
	id := uint(0)
	if _, err := fmt.Sscanf(originalModelID, "%d", &id); err != nil {
		return 0, fmt.Errorf("无效的OriginalModelID格式: %s", originalModelID)
	}

	return id, nil
}

// selectBestHardware 从支持的硬件列表中选择最佳硬件
func (s *taskDeploymentService) selectBestHardware(supportedHardware []string) string {
	// 硬件优先级：bm1684x > bm1684 > 其他
	hardwarePriority := map[string]int{
		"bm1684x": 100,
		"bm1684":  80,
		"bm1688":  90, // 如果有的话
	}

	bestHardware := ""
	bestPriority := -1

	for _, hardware := range supportedHardware {
		// 转换为小写进行比较
		hwLower := strings.ToLower(hardware)
		if priority, exists := hardwarePriority[hwLower]; exists && priority > bestPriority {
			bestHardware = hwLower
			bestPriority = priority
		}
	}

	return bestHardware
}

// isHardwareCompatible 检查硬件平台是否兼容
func (s *taskDeploymentService) isHardwareCompatible(targetChip, requiredChip string) bool {
	// 将硬件名称转换为小写进行比较
	target := strings.ToLower(targetChip)
	required := strings.ToLower(requiredChip)

	// 完全匹配
	if target == required {
		return true
	}

	// 定义硬件兼容性规则
	compatibilityMap := map[string][]string{
		"bm1684x": {"bm1684"}, // bm1684x 可以兼容 bm1684 的模型
		"bm1688":  {"bm1684"}, // bm1688 也可以兼容 bm1684 的模型
	}

	// 检查是否在兼容列表中
	if compatibleList, exists := compatibilityMap[required]; exists {
		for _, compatible := range compatibleList {
			if target == compatible {
				log.Printf("[TaskDeploymentService] Hardware compatibility found: %s can use %s models", required, target)
				return true
			}
		}
	}

	return false
}

// extractTargetChipFromModel 从转换后模型中提取目标芯片信息
func (s *taskDeploymentService) extractTargetChipFromModel(convertedModel *models.ConvertedModel) string {
	// 方法1: 从模型名称中提取，模型名称格式通常为 {name}_{chip}_{quantization}
	// 例如: yolo_v5_bm1684x_f16
	if convertedModel.Name != "" {
		parts := strings.Split(convertedModel.Name, "_")
		for _, part := range parts {
			// 检查是否为已知的芯片类型
			partLower := strings.ToLower(part)
			if partLower == "bm1684x" || partLower == "bm1684" || partLower == "bm1688" {
				return partLower
			}
		}
	}

	// 方法2: 从转换参数中提取
	if convertedModel.ConvertParams != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(convertedModel.ConvertParams), &params); err == nil {
			if targetChip, exists := params["target_chip"]; exists {
				if chipStr, ok := targetChip.(string); ok {
					return strings.ToLower(chipStr)
				}
			}
		}
	}

	// 方法3: 从文件路径中提取，路径通常包含芯片信息
	// 例如: /data/models/converted/yolo-v5-bm1684x.bmodel
	if convertedModel.FilePath != "" {
		fileName := filepath.Base(convertedModel.FilePath)
		fileNameLower := strings.ToLower(fileName)
		if strings.Contains(fileNameLower, "bm1684x") {
			return "bm1684x"
		}
		if strings.Contains(fileNameLower, "bm1684") {
			return "bm1684"
		}
		if strings.Contains(fileNameLower, "bm1688") {
			return "bm1688"
		}
	}

	log.Printf("[TaskDeploymentService] Warning: Could not extract target chip from model: %s", convertedModel.Name)
	return ""
}

// selectModelForBox 根据盒子硬件选择合适的模型（向后兼容方法）
func (s *taskDeploymentService) selectModelForBox(ctx context.Context, originalModelName string, supportedHardware []string) (string, error) {
	log.Printf("[TaskDeploymentService] selectModelForBox (legacy) - OriginalModel: %s, SupportedHardware: %v", originalModelName, supportedHardware)

	// 如果没有硬件支持信息，返回原始模型名
	if len(supportedHardware) == 0 {
		log.Printf("[TaskDeploymentService] No hardware support info, using original model: %s", originalModelName)
		return originalModelName, nil
	}

	// 选择最佳硬件
	selectedHardware := s.selectBestHardware(supportedHardware)
	if selectedHardware == "" {
		log.Printf("[TaskDeploymentService] No supported hardware found, using original model: %s", originalModelName)
		return originalModelName, nil
	}

	// 构建转换后的模型名
	// 格式：converted-{hardware}-{precision}-{originalModelName}
	// 默认使用int8精度
	convertedModelName := fmt.Sprintf("converted-%s-int8-%s", selectedHardware, originalModelName)

	// TODO: 这里应该查询数据库检查转换后的模型是否存在
	// 如果不存在，应该触发模型转换流程
	// 目前简化处理，直接返回转换后的模型名

	log.Printf("[TaskDeploymentService] Selected converted model: %s for hardware: %s", convertedModelName, selectedHardware)
	return convertedModelName, nil
}

// ensureModelAvailableOnBox 确保模型在盒子上可用
func (s *taskDeploymentService) ensureModelAvailableOnBox(ctx context.Context, boxClient *client.BoxClient, modelKey string, inferenceTask *models.InferenceTask, box *models.Box) error {
	log.Printf("[TaskDeploymentService] ensureModelAvailableOnBox started - ModelKey: %s, Box: %d", modelKey, box.ID)

	// 获取盒子上已安装的模型Key列表
	installedModelKeys, err := boxClient.GetInstalledModelKeys(ctx)
	if err != nil {
		log.Printf("[TaskDeploymentService] Failed to get installed models: %v", err)
		// 不阻断流程，直接继续上传模型
		log.Printf("[TaskDeploymentService] Continuing with model upload due to error")
	} else {
		// 检查模型是否已存在
		for _, installedModelKey := range installedModelKeys {
			if installedModelKey == modelKey {
				log.Printf("[TaskDeploymentService] Model %s already exists on box %d", modelKey, box.ID)
				return nil
			}
		}
		log.Printf("[TaskDeploymentService] Model %s not found in installed models: %v", modelKey, installedModelKeys)
	}

	// 模型不存在，需要上传
	log.Printf("[TaskDeploymentService] Model %s not found on box %d, attempting upload", modelKey, box.ID)

	// 上传模型到盒子
	if err := s.uploadModelToBox(ctx, boxClient, modelKey, inferenceTask, box); err != nil {
		return fmt.Errorf("上传模型到盒子失败: %w", err)
	}

	log.Printf("[TaskDeploymentService] Model %s successfully ensured available on box %d", modelKey, box.ID)
	return nil
}

// uploadModelToBox 上传模型到盒子
func (s *taskDeploymentService) uploadModelToBox(ctx context.Context, boxClient *client.BoxClient, modelKey string, inferenceTask *models.InferenceTask, box *models.Box) error {
	log.Printf("[TaskDeploymentService] uploadModelToBox started - ModelKey: %s", modelKey)

	// 如果有OriginalModelID，获取原始模型信息
	if inferenceTask.OriginalModelID != "" {
		return s.uploadModelFromOriginalModel(ctx, boxClient, modelKey, inferenceTask, box)
	}

	// 向后兼容：如果没有OriginalModelID，尝试传统方式
	return s.uploadModelLegacy(ctx, boxClient, modelKey, inferenceTask, box)
}

// uploadModelFromOriginalModel 基于OriginalModelID上传模型
func (s *taskDeploymentService) uploadModelFromOriginalModel(ctx context.Context, boxClient *client.BoxClient, modelKey string, inferenceTask *models.InferenceTask, box *models.Box) error {
	// 解析OriginalModelID
	originalModelIDUint, err := s.parseOriginalModelID(inferenceTask.OriginalModelID)
	if err != nil {
		return fmt.Errorf("解析OriginalModelID失败: %w", err)
	}

	// 获取原始模型信息
	originalModel, err := s.originalModelRepo.GetByID(ctx, originalModelIDUint)
	if err != nil {
		return fmt.Errorf("获取原始模型失败: %w", err)
	}
	if originalModel == nil {
		return fmt.Errorf("原始模型不存在: %d", originalModelIDUint)
	}

	// 查找对应的转换后模型
	convertedModel, err := s.findConvertedModelForBox(ctx, originalModel, box)
	if err != nil {
		return fmt.Errorf("查找转换后模型失败: %w", err)
	}

	if convertedModel == nil {
		return fmt.Errorf("未找到适用于硬件 %v 的转换后模型", box.Meta.SupportedHardware)
	}

	// 执行模型上传
	return s.performModelUpload(ctx, boxClient, modelKey, convertedModel, box)
}

// uploadModelLegacy 传统方式上传模型（向后兼容）
func (s *taskDeploymentService) uploadModelLegacy(ctx context.Context, boxClient *client.BoxClient, modelKey string, inferenceTask *models.InferenceTask, box *models.Box) error {
	log.Printf("[TaskDeploymentService] uploadModelLegacy - ModelKey: %s, ModelName: %s", modelKey, inferenceTask.ModelName)

	// 这里可以实现传统的模型查找和上传逻辑
	// 目前返回一个提示错误
	return fmt.Errorf("模型 %s 不存在于盒子上，且未配置OriginalModelID，无法自动上传", modelKey)
}

// findConvertedModelForBox 查找适用于盒子的转换后模型
func (s *taskDeploymentService) findConvertedModelForBox(ctx context.Context, originalModel *models.OriginalModel, box *models.Box) (*models.ConvertedModel, error) {
	// 选择最佳硬件
	selectedHardware := s.selectBestHardware(box.Meta.SupportedHardware)
	if selectedHardware == "" {
		return nil, fmt.Errorf("盒子不支持任何已知硬件类型")
	}

	log.Printf("[TaskDeploymentService] Looking for converted model - OriginalModelID: %d, Hardware: %s", originalModel.ID, selectedHardware)

	// 检查是否有可用的 ConvertedModelRepository
	if s.convertedModelRepo == nil {
		log.Printf("[TaskDeploymentService] ConvertedModelRepository not available, cannot find converted model for hardware: %s", selectedHardware)
		return nil, fmt.Errorf("ConvertedModelRepository未配置，无法查找转换后模型")
	}

	// 查找对应的转换后模型
	convertedModels, err := s.convertedModelRepo.GetByOriginalModelID(ctx, originalModel.ID)
	if err != nil {
		return nil, fmt.Errorf("查询转换后模型失败: %w", err)
	}

	// 从结果中选择适合的硬件平台的模型
	for _, convertedModel := range convertedModels {
		if convertedModel.Status != models.ConvertedModelStatusCompleted {
			continue
		}

		// 从模型名称或转换参数中提取目标芯片信息
		targetChip := s.extractTargetChipFromModel(convertedModel)
		if targetChip == "" {
			log.Printf("[TaskDeploymentService] Cannot extract target chip from model: %s", convertedModel.Name)
			continue
		}

		// 检查硬件平台是否匹配
		if strings.EqualFold(targetChip, selectedHardware) {
			log.Printf("[TaskDeploymentService] Found suitable converted model: %s for hardware: %s", convertedModel.Name, selectedHardware)
			return convertedModel, nil
		}
	}

	// 如果没找到完全匹配的，尝试查找兼容的模型
	for _, convertedModel := range convertedModels {
		if convertedModel.Status != models.ConvertedModelStatusCompleted {
			continue
		}

		targetChip := s.extractTargetChipFromModel(convertedModel)
		if targetChip == "" {
			continue
		}

		// 检查是否为兼容的硬件平台
		if s.isHardwareCompatible(targetChip, selectedHardware) {
			log.Printf("[TaskDeploymentService] Found compatible converted model: %s (target: %s, required: %s)",
				convertedModel.Name, targetChip, selectedHardware)
			return convertedModel, nil
		}
	}

	log.Printf("[TaskDeploymentService] No suitable converted model found for OriginalModelID: %d, Hardware: %s", originalModel.ID, selectedHardware)
	return nil, fmt.Errorf("未找到适用于硬件 %s 的转换后模型", selectedHardware)
}

// performModelUpload 执行模型上传到盒子
func (s *taskDeploymentService) performModelUpload(ctx context.Context, boxClient *client.BoxClient, modelKey string, convertedModel *models.ConvertedModel, box *models.Box) error {
	log.Printf("[TaskDeploymentService] performModelUpload started - ModelKey: %s, FilePath: %s", modelKey, convertedModel.FilePath)

	// 使用Box-App API上传模型，参考apis.md中的POST /api/v1/models/upload
	uploadRequest := &client.ModelUploadRequest{
		ModelName: modelKey,
		Type:      string(convertedModel.TaskType),
		Version:   convertedModel.Version,
		Hardware:  s.selectBestHardware(box.Meta.SupportedHardware),
		FilePath:  convertedModel.FilePath,
		MD5Sum:    convertedModel.FileMD5,
	}

	// 调用盒子的模型上传API
	if err := boxClient.UploadModel(ctx, uploadRequest); err != nil {
		return fmt.Errorf("调用盒子上传API失败: %w", err)
	}

	log.Printf("[TaskDeploymentService] Model %s uploaded successfully to box %d", modelKey, box.ID)
	return nil
}

// BatchDeployTask 批量下发任务到多个盒子
func (s *taskDeploymentService) BatchDeployTask(ctx context.Context, taskID uint, boxIDs []uint) ([]*DeploymentResponse, error) {
	var responses []*DeploymentResponse
	for _, boxID := range boxIDs {
		resp, err := s.DeployTask(ctx, taskID, boxID)
		if err != nil {
			resp = &DeploymentResponse{
				TaskID:    taskID,
				BoxID:     boxID,
				Success:   false,
				Message:   fmt.Sprintf("部署失败: %v", err),
				ErrorCode: "DEPLOYMENT_FAILED",
			}
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

// UpdateTaskOnBox 更新盒子上的任务配置
func (s *taskDeploymentService) UpdateTaskOnBox(ctx context.Context, taskID uint, boxID uint) (*DeploymentResponse, error) {
	log.Printf("[TaskDeploymentService] UpdateTaskOnBox started - TaskID: %d, BoxID: %d", taskID, boxID)

	// 获取任务信息
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("获取任务失败: %v", err),
			ErrorCode: "TASK_NOT_FOUND",
		}, err
	}

	// 获取盒子信息
	box, err := s.boxRepo.GetByID(ctx, boxID)
	if err != nil {
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("获取盒子失败: %v", err),
			ErrorCode: "BOX_NOT_FOUND",
		}, err
	}

	// 检查盒子状态
	if box.Status != models.BoxStatusOnline {
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("盒子不在线，当前状态: %s", box.Status),
			ErrorCode: "BOX_OFFLINE",
		}, fmt.Errorf("盒子不在线")
	}

	// 创建盒子客户端
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))

	// 检查盒子健康状态
	if err := boxClient.Health(ctx); err != nil {
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("盒子健康检查失败: %v", err),
			ErrorCode: "BOX_HEALTH_CHECK_FAILED",
		}, err
	}

	// 转换任务配置
	boxTask, err := s.convertToBoxTask(ctx, task, box)
	if err != nil {
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("转换任务配置失败: %v", err),
			ErrorCode: "TASK_CONVERSION_FAILED",
		}, err
	}

	// 更新盒子上的任务配置
	if err := boxClient.UpdateTask(ctx, task.TaskID, boxTask); err != nil {
		return &DeploymentResponse{
			TaskID:    taskID,
			BoxID:     boxID,
			Success:   false,
			Status:    TaskDeploymentStatusFailed,
			Message:   fmt.Sprintf("更新盒子任务失败: %v", err),
			ErrorCode: "BOX_UPDATE_FAILED",
		}, err
	}

	// 生成执行ID
	executionID := fmt.Sprintf("update_%d_%d_%d", taskID, boxID, time.Now().Unix())

	log.Printf("[TaskDeploymentService] Task %d successfully updated on box %d", taskID, boxID)
	return &DeploymentResponse{
		TaskID:      taskID,
		BoxID:       boxID,
		Success:     true,
		Status:      TaskDeploymentStatusSuccess,
		ExecutionID: executionID,
		Message:     "任务配置更新成功",
	}, nil
}

// StopTaskOnBox 停止盒子上的任务
func (s *taskDeploymentService) StopTaskOnBox(ctx context.Context, taskID uint, boxID uint) error {
	log.Printf("[TaskDeploymentService] StopTaskOnBox started - TaskID: %d, BoxID: %d", taskID, boxID)

	// 获取任务信息
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}

	// 获取盒子信息
	box, err := s.boxRepo.GetByID(ctx, boxID)
	if err != nil {
		return fmt.Errorf("获取盒子失败: %w", err)
	}

	// 创建盒子客户端
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))

	// 检查盒子健康状态
	if err := boxClient.Health(ctx); err != nil {
		log.Printf("[TaskDeploymentService] Box health check failed, but continuing with stop: %v", err)
		// 盒子不健康时仍继续停止操作，因为可能盒子重启了
	}

	// 尝试停止任务
	if err := boxClient.StopTask(ctx, task.TaskID); err != nil {
		log.Printf("[TaskDeploymentService] Failed to stop task on box: %v", err)
		// 如果停止失败，尝试删除任务
		if deleteErr := boxClient.DeleteTask(ctx, task.TaskID); deleteErr != nil {
			log.Printf("[TaskDeploymentService] Failed to delete task on box: %v", deleteErr)
			return fmt.Errorf("停止任务失败，删除任务也失败: stop_error=%v, delete_error=%v", err, deleteErr)
		}
		log.Printf("[TaskDeploymentService] Task deleted from box after stop failure")
	} else {
		log.Printf("[TaskDeploymentService] Task stopped successfully on box")

		// 停止成功后也删除任务
		if err := boxClient.DeleteTask(ctx, task.TaskID); err != nil {
			log.Printf("[TaskDeploymentService] Failed to delete task after stop: %v", err)
			// 删除失败不返回错误，因为任务已经停止
		}
	}

	log.Printf("[TaskDeploymentService] Task %d stopped on box %d", taskID, boxID)
	return nil
}

// GetTaskStatusFromBox 从盒子获取任务状态
func (s *taskDeploymentService) GetTaskStatusFromBox(ctx context.Context, taskID uint, boxID uint) (*BoxTaskStatus, error) {
	log.Printf("[TaskDeploymentService] GetTaskStatusFromBox started - TaskID: %d, BoxID: %d", taskID, boxID)

	// 获取任务信息
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	// 获取盒子信息
	box, err := s.boxRepo.GetByID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("获取盒子失败: %w", err)
	}

	// 创建盒子客户端
	boxClient := client.NewBoxClient(box.IPAddress, int(box.Port))

	// 检查盒子健康状态
	if err := boxClient.Health(ctx); err != nil {
		log.Printf("[TaskDeploymentService] Box health check failed: %v", err)
		return &BoxTaskStatus{
			TaskID:      taskID,
			Status:      "box_offline",
			Progress:    0,
			Error:       fmt.Sprintf("盒子连接失败: %v", err),
			Message:     "盒子不可访问",
			LastUpdated: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	// 获取任务状态
	boxTaskStatus, err := boxClient.GetTaskStatus(ctx, task.TaskID)
	if err != nil {
		log.Printf("[TaskDeploymentService] Failed to get task status from box: %v", err)
		return &BoxTaskStatus{
			TaskID:      taskID,
			Status:      "error",
			Progress:    0,
			Error:       fmt.Sprintf("获取任务状态失败: %v", err),
			Message:     "无法获取盒子上的任务状态",
			LastUpdated: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	// 转换盒子返回的状态
	status := &BoxTaskStatus{
		TaskID:      taskID,
		Status:      boxTaskStatus.Status,
		Progress:    boxTaskStatus.Progress,
		Error:       boxTaskStatus.Error,
		Message:     boxTaskStatus.Message,
		LastUpdated: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 如果盒子返回了统计信息，转换格式
	if boxTaskStatus.Statistics != nil {
		status.Statistics = &TaskStatistics{
			FPS:             boxTaskStatus.Statistics.FPS,
			AverageLatency:  boxTaskStatus.Statistics.AverageLatency,
			ProcessedFrames: boxTaskStatus.Statistics.ProcessedFrames,
			TotalFrames:     boxTaskStatus.Statistics.TotalFrames,
			InferenceCount:  boxTaskStatus.Statistics.InferenceCount,
			ForwardSuccess:  boxTaskStatus.Statistics.ForwardSuccess,
			ForwardFailed:   boxTaskStatus.Statistics.ForwardFailed,
		}
	}

	log.Printf("[TaskDeploymentService] Task status retrieved from box - Status: %s, Progress: %.2f", status.Status, status.Progress)
	return status, nil
}

// ValidateTaskConfig 验证任务配置
func (s *taskDeploymentService) ValidateTaskConfig(ctx context.Context, task *models.Task) error {
	log.Printf("[TaskDeploymentService] ValidateTaskConfig started - TaskID: %s", task.TaskID)

	// 验证基础字段
	if task.TaskID == "" {
		return fmt.Errorf("任务ID不能为空")
	}
	if task.Name == "" {
		return fmt.Errorf("任务名称不能为空")
	}
	if task.VideoSourceID == 0 {
		return fmt.Errorf("视频源ID不能为空")
	}

	// 验证视频源是否存在
	videoSource, err := s.videoSourceRepo.GetByID(task.VideoSourceID)
	if err != nil {
		return fmt.Errorf("视频源不存在: %w", err)
	}
	if videoSource.Status != models.VideoSourceStatusActive {
		return fmt.Errorf("视频源状态异常: %s", videoSource.Status)
	}

	// 验证推理任务配置
	if len(task.InferenceTasks) == 0 {
		return fmt.Errorf("至少需要配置一个推理任务")
	}

	for i, inferenceTask := range task.InferenceTasks {
		// 验证推理任务类型
		if inferenceTask.Type == "" {
			return fmt.Errorf("推理任务[%d]: 推理类型不能为空", i)
		}

		// 验证模型配置 - 现在只检查OriginalModelID或ModelName
		if inferenceTask.OriginalModelID == "" && inferenceTask.ModelName == "" {
			return fmt.Errorf("推理任务[%d]: 必须指定原始模型ID或模型名", i)
		}

		// 验证阈值范围
		if inferenceTask.Threshold < 0 || inferenceTask.Threshold > 1 {
			return fmt.Errorf("推理任务[%d]: 阈值必须在0-1范围内", i)
		}

		// 验证转发配置
		if len(inferenceTask.ForwardInfos) > 0 {
			for j, forwardInfo := range inferenceTask.ForwardInfos {
				if err := s.validateForwardInfo(&forwardInfo, i, j); err != nil {
					return err
				}
			}
		}

		// 向后兼容：验证单个ForwardInfo
		if inferenceTask.ForwardInfo != nil {
			if err := s.validateForwardInfo(inferenceTask.ForwardInfo, i, -1); err != nil {
				return err
			}
		}
	}

	// 验证ROI配置
	for i, roi := range task.ROIs {
		if roi.Width <= 0 || roi.Height <= 0 {
			return fmt.Errorf("ROI[%d]: 宽度和高度必须大于0", i)
		}
		if roi.X < 0 || roi.Y < 0 {
			return fmt.Errorf("ROI[%d]: 坐标不能为负数", i)
		}
	}

	// 验证优先级范围
	if task.Priority < 1 || task.Priority > 5 {
		return fmt.Errorf("优先级必须在1-5范围内")
	}

	// 验证最大重试次数
	if task.MaxRetries < 0 || task.MaxRetries > 10 {
		return fmt.Errorf("最大重试次数必须在0-10范围内")
	}

	log.Printf("[TaskDeploymentService] Task config validation passed - TaskID: %s", task.TaskID)
	return nil
}

// validateForwardInfo 验证转发配置
func (s *taskDeploymentService) validateForwardInfo(forwardInfo *models.ForwardInfo, taskIndex, forwardIndex int) error {
	if forwardInfo.Type == "" {
		if forwardIndex >= 0 {
			return fmt.Errorf("推理任务[%d]转发配置[%d]: 转发类型不能为空", taskIndex, forwardIndex)
		} else {
			return fmt.Errorf("推理任务[%d]转发配置: 转发类型不能为空", taskIndex)
		}
	}

	// 验证支持的转发类型
	supportedTypes := []string{"mqtt", "http_post", "websocket"}
	typeSupported := false
	for _, supportedType := range supportedTypes {
		if forwardInfo.Type == supportedType {
			typeSupported = true
			break
		}
	}
	if !typeSupported {
		return fmt.Errorf("推理任务[%d]: 不支持的转发类型 %s，支持的类型: %v", taskIndex, forwardInfo.Type, supportedTypes)
	}

	// 验证主机地址
	if forwardInfo.Host == "" {
		return fmt.Errorf("推理任务[%d]: 转发主机地址不能为空", taskIndex)
	}

	// 验证端口范围
	if forwardInfo.Port <= 0 || forwardInfo.Port > 65535 {
		return fmt.Errorf("推理任务[%d]: 端口必须在1-65535范围内", taskIndex)
	}

	// 特定类型的验证
	switch forwardInfo.Type {
	case "mqtt":
		if forwardInfo.Topic == "" {
			return fmt.Errorf("推理任务[%d]: MQTT转发必须指定主题", taskIndex)
		}
	case "http_post":
		if forwardInfo.Topic == "" {
			return fmt.Errorf("推理任务[%d]: HTTP POST转发必须指定路径", taskIndex)
		}
	}

	return nil
}
