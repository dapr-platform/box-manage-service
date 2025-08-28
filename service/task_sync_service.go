/*
 * @module service/task_sync_service
 * @description 任务同步服务，用于同步盒子的任务到管理端
 * @architecture 服务层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow API调用盒子 -> 获取任务列表 -> 与本地对比 -> 创建/更新任务
 * @rules 提供幂等的任务同步操作，支持一键同步盒子所有任务
 * @dependencies repository, box_proxy
 * @refs apis.md
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// TaskSyncService 任务同步服务
type TaskSyncService struct {
	repoManager  repository.RepositoryManager
	proxyService *BoxProxyService
}

// NewTaskSyncService 创建任务同步服务
func NewTaskSyncService(repoManager repository.RepositoryManager, proxyService *BoxProxyService) *TaskSyncService {
	return &TaskSyncService{
		repoManager:  repoManager,
		proxyService: proxyService,
	}
}

// SyncBoxTasks 同步指定盒子的所有任务
func (s *TaskSyncService) SyncBoxTasks(ctx context.Context, boxID uint) (*TaskSyncResult, error) {
	// 获取盒子信息
	box, err := s.repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("获取盒子信息失败: %w", err)
	}

	if box.Status != models.BoxStatusOnline {
		return nil, fmt.Errorf("盒子 %s 不在线，无法同步任务", box.Name)
	}

	// 从盒子获取任务列表
	tasksResp, err := s.proxyService.GetTasks(boxID)
	if err != nil {
		return nil, fmt.Errorf("获取盒子任务列表失败: %w", err)
	}

	if tasksResp == nil || tasksResp.Tasks == nil {
		return &TaskSyncResult{
			BoxID:        boxID,
			BoxName:      box.Name,
			TotalTasks:   0,
			SyncedTasks:  0,
			UpdatedTasks: 0,
			SkippedTasks: 0,
			ErrorTasks:   0,
		}, nil
	}

	result := &TaskSyncResult{
		BoxID:        boxID,
		BoxName:      box.Name,
		TotalTasks:   len(tasksResp.Tasks),
		SyncedTasks:  0,
		UpdatedTasks: 0,
		SkippedTasks: 0,
		ErrorTasks:   0,
		Details:      make([]TaskSyncDetail, 0),
	}

	// 遍历盒子上的任务，逐个同步
	for _, boxTask := range tasksResp.Tasks {
		detail := s.syncSingleTask(ctx, boxID, boxTask)
		result.Details = append(result.Details, detail)

		switch detail.Action {
		case "created":
			result.SyncedTasks++
		case "updated":
			result.UpdatedTasks++
		case "skipped":
			result.SkippedTasks++
		case "error":
			result.ErrorTasks++
		}
	}

	return result, nil
}

// syncSingleTask 同步单个任务
func (s *TaskSyncService) syncSingleTask(ctx context.Context, boxID uint, boxTask TaskInfo) TaskSyncDetail {
	detail := TaskSyncDetail{
		ExternalID: boxTask.TaskID,
		TaskName:   boxTask.TaskID, // 使用任务ID作为默认名称
		Status:     boxTask.Status,
	}

	// 检查任务是否已经存在
	existingTask, err := s.repoManager.Task().FindByExternalID(ctx, boxID, boxTask.TaskID)
	if err != nil {
		detail.Action = "error"
		detail.Error = fmt.Sprintf("查询现有任务失败: %v", err)
		log.Printf("同步任务 %s 时查询失败: %v", boxTask.TaskID, err)
		return detail
	}

	if existingTask != nil {
		// 任务已存在，更新状态和基本信息
		detail.Action = "updated"
		detail.TaskID = existingTask.TaskID

		// 更新任务状态和基本信息
		existingTask.Status = s.mapBoxTaskStatus(boxTask.Status)
		existingTask.AutoStart = boxTask.AutoStart
		existingTask.SkipFrame = boxTask.SkipFrame
		existingTask.IsSynchronized = true
		existingTask.UpdatedAt = time.Now()

		if err := s.repoManager.Task().Update(ctx, existingTask); err != nil {
			detail.Action = "error"
			detail.Error = fmt.Sprintf("更新任务失败: %v", err)
			log.Printf("更新任务 %s 失败: %v", boxTask.TaskID, err)
		}
		return detail
	}

	// 任务不存在，需要获取详细信息并创建
	taskDetail, err := s.proxyService.GetTask(boxID, boxTask.TaskID)
	if err != nil {
		detail.Action = "error"
		detail.Error = fmt.Sprintf("获取任务详情失败: %v", err)
		log.Printf("获取任务 %s 详情失败: %v", boxTask.TaskID, err)
		return detail
	}

	if taskDetail == nil || taskDetail.Task.TaskID == "" {
		detail.Action = "error"
		detail.Error = "任务详情为空"
		return detail
	}

	// 获取或创建对应的视频源
	videoSourceID, err := s.getOrCreateVideoSourceFromRTSP(ctx, taskDetail.Task.RTSPUrl, taskDetail.Task.TaskID)
	if err != nil {
		detail.Action = "error"
		detail.Error = fmt.Sprintf("获取或创建视频源失败: %v", err)
		log.Printf("获取或创建视频源失败: %v", err)
		return detail
	}

	// 创建新任务
	newTask := &models.Task{
		Name:           taskDetail.Task.TaskID, // 使用任务ID作为名称
		Description:    fmt.Sprintf("从盒子 %d 同步的任务", boxID),
		BoxID:          &boxID,
		VideoSourceID:  videoSourceID,
		TaskID:         s.generateUniqueTaskID(ctx, boxID, taskDetail.Task.TaskID),
		SkipFrame:      taskDetail.Task.SkipFrame,
		AutoStart:      taskDetail.Task.AutoStart,
		Status:         s.mapBoxTaskStatus(boxTask.Status),
		Source:         "synced",
		IsSynchronized: true,
	}

	// 转换任务配置
	if err := s.convertTaskConfiguration(taskDetail.Task, newTask); err != nil {
		detail.Action = "error"
		detail.Error = fmt.Sprintf("转换任务配置失败: %v", err)
		log.Printf("转换任务 %s 配置失败: %v", boxTask.TaskID, err)
		return detail
	}

	// 创建任务
	if err := s.repoManager.Task().Create(ctx, newTask); err != nil {
		detail.Action = "error"
		detail.Error = fmt.Sprintf("创建任务失败: %v", err)
		log.Printf("创建任务 %s 失败: %v", boxTask.TaskID, err)
		return detail
	}

	detail.Action = "created"
	detail.TaskID = newTask.TaskID
	log.Printf("成功同步任务: %s -> %s", boxTask.TaskID, newTask.TaskID)

	return detail
}

// generateUniqueTaskID 生成唯一的任务ID
func (s *TaskSyncService) generateUniqueTaskID(ctx context.Context, boxID uint, externalID string) string {
	baseID := fmt.Sprintf("sync_%d_%s", boxID, externalID)

	// 检查是否重复
	if existing, _ := s.repoManager.Task().FindByTaskID(ctx, baseID); existing == nil {
		return baseID
	}

	// 如果重复，添加时间戳
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s_%d", baseID, timestamp)
}

// mapBoxTaskStatus 映射盒子任务状态到管理端状态
func (s *TaskSyncService) mapBoxTaskStatus(boxStatus string) models.TaskStatus {
	switch strings.ToLower(boxStatus) {
	case "running":
		return models.TaskStatusRunning
	case "stopped":
		return models.TaskStatusStopped
	case "starting":
		return models.TaskStatusScheduled
	case "stopping":
		return models.TaskStatusStopping
	case "error":
		return models.TaskStatusFailed
	default:
		return models.TaskStatusPending
	}
}

// convertTaskConfiguration 转换任务配置
func (s *TaskSyncService) convertTaskConfiguration(boxTask TaskDetailInfo, task *models.Task) error {
	// 转换输出设置
	if boxTask.OutputSettings != nil {
		outputSettings := models.OutputSettings{}
		if data, err := json.Marshal(boxTask.OutputSettings); err == nil {
			json.Unmarshal(data, &outputSettings)
		}
		task.OutputSettings = outputSettings
	}

	// 转换ROI配置
	if boxTask.ROIs != nil {
		rois := models.ROIConfigList{}
		if data, err := json.Marshal(boxTask.ROIs); err == nil {
			json.Unmarshal(data, &rois)
		}
		task.ROIs = rois
	}

	// 转换推理任务配置
	if boxTask.InferenceTasks != nil {
		inferenceTasks := models.InferenceTaskList{}
		if data, err := json.Marshal(boxTask.InferenceTasks); err == nil {
			json.Unmarshal(data, &inferenceTasks)
		}
		task.InferenceTasks = inferenceTasks
	}

	return nil
}

// TaskSyncResult 任务同步结果
type TaskSyncResult struct {
	BoxID        uint             `json:"box_id"`
	BoxName      string           `json:"box_name"`
	TotalTasks   int              `json:"total_tasks"`
	SyncedTasks  int              `json:"synced_tasks"`
	UpdatedTasks int              `json:"updated_tasks"`
	SkippedTasks int              `json:"skipped_tasks"`
	ErrorTasks   int              `json:"error_tasks"`
	Details      []TaskSyncDetail `json:"details"`
	SyncTime     time.Time        `json:"sync_time"`
}

// TaskSyncDetail 单个任务同步详情
type TaskSyncDetail struct {
	ExternalID string `json:"external_id"`
	TaskID     string `json:"task_id"`
	TaskName   string `json:"task_name"`
	Status     string `json:"status"`
	Action     string `json:"action"` // created, updated, skipped, error
	Error      string `json:"error,omitempty"`
}

// getOrCreateVideoSourceFromRTSP 获取或创建对应的视频源
func (s *TaskSyncService) getOrCreateVideoSourceFromRTSP(ctx context.Context, rtspUrl, taskID string) (uint, error) {
	// 尝试根据URL查找现有的视频源
	videoSources, err := s.repoManager.VideoSource().FindByURL(rtspUrl)
	if err != nil {
		return 0, fmt.Errorf("查找视频源失败: %w", err)
	}

	if len(videoSources) > 0 {
		return videoSources[0].ID, nil
	}

	// 没有找到现有的视频源，创建新的
	videoSource := &models.VideoSource{
		Name:        fmt.Sprintf("同步-%s", taskID),
		Description: fmt.Sprintf("从任务 %s 同步的视频源", taskID),
		Type:        models.VideoSourceTypeStream, // 假设RTSP是实时流
		URL:         rtspUrl,
		PlayURL:     rtspUrl,
		Status:      models.VideoSourceStatusActive,
	}

	if err := s.repoManager.VideoSource().Create(videoSource); err != nil {
		return 0, fmt.Errorf("创建视频源失败: %w", err)
	}

	return videoSource.ID, nil
}
