/*
 * @module service/box_client_service
 * @description 盒子客户端服务，处理盒子端发送的心跳数据和任务同步
 * @architecture 服务层
 * @documentReference REQ-001: 盒子管理功能, req_1222.md
 * @stateFlow 心跳接收 -> 盒子识别 -> 状态更新 -> 任务同步
 * @rules 提供心跳处理、盒子状态更新、任务同步等功能
 * @dependencies repository, box_proxy, task_sync
 * @refs req_1222.md
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// BoxClientService 盒子客户端服务
type BoxClientService struct {
	repoManager     repository.RepositoryManager
	proxyService    *BoxProxyService
	taskSyncService *TaskSyncService
	syncLocks       sync.Map // 用于防止同一盒子的任务同步并发执行，key为boxID
}

// NewBoxClientService 创建盒子客户端服务
func NewBoxClientService(repoManager repository.RepositoryManager, proxyService *BoxProxyService, taskSyncService *TaskSyncService) *BoxClientService {
	return &BoxClientService{
		repoManager:     repoManager,
		proxyService:    proxyService,
		taskSyncService: taskSyncService,
	}
}

// acquireSyncLock 尝试获取盒子的同步锁
func (s *BoxClientService) acquireSyncLock(boxID uint) bool {
	_, loaded := s.syncLocks.LoadOrStore(boxID, true)
	return !loaded // 如果是新存储的，返回true；如果已经存在，返回false
}

// releaseSyncLock 释放盒子的同步锁
func (s *BoxClientService) releaseSyncLock(boxID uint) {
	s.syncLocks.Delete(boxID)
}

// ProcessHeartbeat 处理心跳数据
// 1. 根据IP地址和设备指纹识别盒子
// 2. 更新盒子状态和属性
// 3. 根据任务信息触发任务同步
func (s *BoxClientService) ProcessHeartbeat(ctx context.Context, heartbeat *models.BoxHeartbeatRequest) (*models.BoxHeartbeatResponse, error) {
	log.Printf("[BoxClientService] 收到心跳数据: IP=%s, Port=%d, DeviceFingerprint=%s",
		heartbeat.IPAddress, heartbeat.Port, heartbeat.Device.DeviceFingerprint)

	// 1. 识别盒子
	box, err := s.identifyBox(ctx, heartbeat)
	if err != nil {
		return nil, fmt.Errorf("识别盒子失败: %w", err)
	}

	// 2. 更新盒子属性和状态
	if err := s.updateBoxFromHeartbeat(ctx, box, heartbeat); err != nil {
		log.Printf("[BoxClientService] 更新盒子状态失败: %v", err)
		// 不返回错误，继续处理
	}

	// 3. 处理任务同步
	tasksToSync, syncTriggered := s.processTaskSync(ctx, box, heartbeat)

	// 构建响应
	response := &models.BoxHeartbeatResponse{
		Success:       true,
		BoxID:         box.ID,
		Timestamp:     time.Now().UnixMilli(),
		Message:       "心跳处理成功",
		TasksToSync:   tasksToSync,
		SyncTriggered: syncTriggered,
	}

	return response, nil
}

// identifyBox 识别盒子（根据IP地址或设备指纹）
func (s *BoxClientService) identifyBox(ctx context.Context, heartbeat *models.BoxHeartbeatRequest) (*models.Box, error) {
	boxRepo := s.repoManager.Box()

	// 优先根据设备指纹查找
	if heartbeat.Device.DeviceFingerprint != "" {
		box, err := boxRepo.FindByDeviceFingerprint(ctx, heartbeat.Device.DeviceFingerprint)
		if err == nil && box != nil {
			log.Printf("[BoxClientService] 通过设备指纹找到盒子: ID=%d, Name=%s", box.ID, box.Name)
			return box, nil
		}
	}

	// 根据IP地址查找
	if heartbeat.IPAddress != "" {
		box, err := boxRepo.FindByIPAddress(ctx, heartbeat.IPAddress)
		if err == nil && box != nil {
			log.Printf("[BoxClientService] 通过IP地址找到盒子: ID=%d, Name=%s", box.ID, box.Name)
			return box, nil
		}
	}

	// 如果没有找到，自动创建新盒子
	log.Printf("[BoxClientService] 未找到匹配的盒子，自动创建新盒子")
	newBox, err := s.createBoxFromHeartbeat(ctx, heartbeat)
	if err != nil {
		return nil, fmt.Errorf("自动创建盒子失败: %w", err)
	}

	return newBox, nil
}

// createBoxFromHeartbeat 根据心跳数据创建新盒子
func (s *BoxClientService) createBoxFromHeartbeat(ctx context.Context, heartbeat *models.BoxHeartbeatRequest) (*models.Box, error) {
	// 生成盒子名称
	name := fmt.Sprintf("Box-%s", heartbeat.Device.DeviceFingerprint[:8])
	if heartbeat.Device.DeviceFingerprint == "" {
		name = fmt.Sprintf("Box-%s-%d", heartbeat.IPAddress, heartbeat.Port)
	}

	box := &models.Box{
		Name:              name,
		IPAddress:         heartbeat.IPAddress,
		Port:              heartbeat.Port,
		Status:            models.BoxStatusOnline,
		ApiKey:            heartbeat.ApiKey,
		DeviceFingerprint: heartbeat.Device.DeviceFingerprint,
		LicenseID:         heartbeat.Device.LicenseID,
		Edition:           heartbeat.Device.Edition,
		IsLicenseValid:    heartbeat.Device.IsValid,
		Version:           heartbeat.Version,
	}

	// 设置心跳时间
	now := time.Now()
	box.LastHeartbeat = &now

	// 设置资源信息
	box.Resources = heartbeat.System.ConvertToResources()

	// 创建盒子
	if err := s.repoManager.Box().Create(ctx, box); err != nil {
		return nil, fmt.Errorf("创建盒子失败: %w", err)
	}

	log.Printf("[BoxClientService] 成功创建新盒子: ID=%d, Name=%s", box.ID, box.Name)
	return box, nil
}

// updateBoxFromHeartbeat 根据心跳数据更新盒子信息
func (s *BoxClientService) updateBoxFromHeartbeat(ctx context.Context, box *models.Box, heartbeat *models.BoxHeartbeatRequest) error {
	// 更新认证信息
	box.ApiKey = heartbeat.ApiKey
	box.DeviceFingerprint = heartbeat.Device.DeviceFingerprint
	box.LicenseID = heartbeat.Device.LicenseID
	box.Edition = heartbeat.Device.Edition
	box.IsLicenseValid = heartbeat.Device.IsValid

	// 更新版本信息
	box.Version = heartbeat.Version

	// 更新端口（如果变化）
	if heartbeat.Port != 0 && heartbeat.Port != box.Port {
		box.Port = heartbeat.Port
	}

	// 更新IP地址（如果变化）
	if heartbeat.IPAddress != "" && heartbeat.IPAddress != box.IPAddress {
		box.IPAddress = heartbeat.IPAddress
	}

	// 更新心跳时间
	now := time.Now()
	box.LastHeartbeat = &now

	// 更新资源信息
	box.Resources = heartbeat.System.ConvertToResources()

	// 更新状态为在线
	box.Status = models.BoxStatusOnline

	// 保存更新
	if err := s.repoManager.Box().Update(ctx, box); err != nil {
		return fmt.Errorf("更新盒子信息失败: %w", err)
	}

	log.Printf("[BoxClientService] 更新盒子信息成功: ID=%d, Name=%s, ApiKey=%s",
		box.ID, box.Name, maskApiKey(box.ApiKey))

	return nil
}

// processTaskSync 处理任务同步
func (s *BoxClientService) processTaskSync(ctx context.Context, box *models.Box, heartbeat *models.BoxHeartbeatRequest) (int, bool) {
	if heartbeat.Tasks.TotalTasks == 0 {
		return 0, false
	}

	// 获取盒子上的任务列表
	reportedTasks := heartbeat.Tasks.Tasks

	// 获取管理端已知的该盒子任务
	existingTasks, err := s.repoManager.Task().FindByBoxID(ctx, box.ID)
	if err != nil {
		log.Printf("[BoxClientService] 获取盒子任务列表失败: %v", err)
		return 0, false
	}

	// 构建已存在任务的ID映射
	existingTaskMap := make(map[string]*models.Task)
	for _, task := range existingTasks {
		// 使用ExternalID进行匹配
		if task.ExternalID != "" {
			existingTaskMap[task.ExternalID] = task
		}
		// 也使用TaskID进行匹配
		existingTaskMap[task.TaskID] = task
	}

	tasksToSync := 0
	syncTriggered := false

	// 遍历心跳中报告的任务
	for _, reportedTask := range reportedTasks {
		// 检查任务是否存在于管理端
		existingTask, exists := existingTaskMap[reportedTask.TaskID]

		if exists {
			// 任务存在，更新状态
			s.updateTaskStatus(ctx, existingTask, &reportedTask)
		} else {
			// 任务不存在，标记为需要同步
			tasksToSync++
		}
	}

	// 如果有需要同步的任务，触发异步同步
	if tasksToSync > 0 {
		syncTriggered = true
		go s.syncNewTasks(context.Background(), box, heartbeat)
	}

	return tasksToSync, syncTriggered
}

// updateTaskStatus 更新任务状态
func (s *BoxClientService) updateTaskStatus(ctx context.Context, task *models.Task, reported *models.HeartbeatTaskStatInfo) {
	// 映射状态
	newStatus := s.mapTaskStatus(reported.Stats.Status)

	// 更新统计信息
	task.TotalFrames = reported.Stats.TotalFrames
	task.InferenceCount = reported.Stats.InferenceCount
	task.ForwardSuccess = reported.Stats.ForwardSuccess
	task.ForwardFailed = reported.Stats.ForwardFailed
	task.LastError = reported.Stats.LastError

	// 更新状态
	if task.Status != newStatus {
		task.Status = newStatus
	}

	// 更新心跳时间
	task.UpdateHeartbeat()

	// 处理启动时间
	if reported.Stats.StartTime > 0 {
		startTime := time.UnixMilli(reported.Stats.StartTime)
		task.StartTime = &startTime
	}

	// 处理停止时间
	if reported.Stats.StopTime > 0 {
		stopTime := time.UnixMilli(reported.Stats.StopTime)
		task.StopTime = &stopTime
	}

	// 标记已同步
	task.IsSynchronized = true

	// 保存更新
	if err := s.repoManager.Task().Update(ctx, task); err != nil {
		log.Printf("[BoxClientService] 更新任务状态失败: TaskID=%s, Error=%v", task.TaskID, err)
	}
}

// syncNewTasks 同步新任务（异步执行）
func (s *BoxClientService) syncNewTasks(ctx context.Context, box *models.Box, heartbeat *models.BoxHeartbeatRequest) {
	// 尝试获取同步锁，防止并发同步
	if !s.acquireSyncLock(box.ID) {
		log.Printf("[BoxClientService] 盒子 %d 的任务同步正在进行中，跳过本次同步", box.ID)
		return
	}
	defer s.releaseSyncLock(box.ID)

	log.Printf("[BoxClientService] 开始同步盒子 %d 上的新任务", box.ID)

	// 获取管理端已知的该盒子任务
	existingTasks, err := s.repoManager.Task().FindByBoxID(ctx, box.ID)
	if err != nil {
		log.Printf("[BoxClientService] 获取盒子任务列表失败: %v", err)
		return
	}

	// 构建已存在任务的ID映射
	existingTaskMap := make(map[string]bool)
	for _, task := range existingTasks {
		if task.ExternalID != "" {
			existingTaskMap[task.ExternalID] = true
		}
		existingTaskMap[task.TaskID] = true
	}

	// 遍历心跳中报告的任务，同步不存在的任务
	for _, reportedTask := range heartbeat.Tasks.Tasks {
		if existingTaskMap[reportedTask.TaskID] {
			continue // 任务已存在，跳过
		}

		// 从盒子获取任务详情
		err := s.syncSingleTask(ctx, box, reportedTask.TaskID, &reportedTask.Stats)
		if err != nil {
			log.Printf("[BoxClientService] 同步任务失败: TaskID=%s, Error=%v", reportedTask.TaskID, err)
			continue
		}

		// 成功同步后，将任务ID添加到映射中，防止同一批次心跳中重复处理
		existingTaskMap[reportedTask.TaskID] = true

		log.Printf("[BoxClientService] 成功同步任务: TaskID=%s", reportedTask.TaskID)
	}

	log.Printf("[BoxClientService] 盒子 %d 任务同步完成", box.ID)
}

// syncSingleTask 同步单个任务
func (s *BoxClientService) syncSingleTask(ctx context.Context, box *models.Box, taskID string, stats *models.HeartbeatTaskStatsData) error {
	// 从盒子API获取任务详情
	taskDetail, err := s.proxyService.GetTask(box.ID, taskID)
	if err != nil {
		return fmt.Errorf("获取任务详情失败: %w", err)
	}

	if taskDetail == nil || taskDetail.Task.TaskID == "" {
		return fmt.Errorf("任务详情为空")
	}

	// 获取或创建视频源
	videoSourceID, err := s.getOrCreateVideoSource(ctx, taskDetail.Task.RTSPUrl, taskID)
	if err != nil {
		return fmt.Errorf("获取或创建视频源失败: %w", err)
	}

	// 同步任务使用的模型
	err = s.syncTaskModels(ctx, box, taskDetail)
	if err != nil {
		log.Printf("[BoxClientService] 同步任务模型时出错: %v", err)
		// 不中断，继续创建任务
	}

	// 创建任务
	newTask := &models.Task{
		Name:           taskDetail.Task.TaskID,
		Description:    fmt.Sprintf("从盒子 %s 同步的任务", box.Name),
		BoxID:          &box.ID,
		VideoSourceID:  videoSourceID,
		TaskID:         s.generateUniqueTaskID(ctx, box.ID, taskID),
		ExternalID:     taskID, // 保存原始任务ID
		SkipFrame:      taskDetail.Task.SkipFrame,
		AutoStart:      taskDetail.Task.AutoStart,
		Status:         s.mapTaskStatus(stats.Status),
		Source:         "synced",
		IsSynchronized: true,
	}

	// 转换配置
	s.convertTaskConfig(taskDetail, newTask)

	// 设置统计信息
	if stats != nil {
		newTask.TotalFrames = stats.TotalFrames
		newTask.InferenceCount = stats.InferenceCount
		newTask.ForwardSuccess = stats.ForwardSuccess
		newTask.ForwardFailed = stats.ForwardFailed
		newTask.LastError = stats.LastError
	}

	// 保存任务
	if err := s.repoManager.Task().Create(ctx, newTask); err != nil {
		return fmt.Errorf("创建任务失败: %w", err)
	}

	return nil
}

// syncTaskModels 同步任务使用的模型
func (s *BoxClientService) syncTaskModels(ctx context.Context, box *models.Box, taskDetail *TaskDetailResponse) error {
	if len(taskDetail.Task.InferenceTasks) == 0 {
		return nil
	}

	// 获取盒子的芯片类型
	chipType := s.getBoxChipType(box)

	for _, inferenceTask := range taskDetail.Task.InferenceTasks {
		// 从map中提取modelKey
		modelKey := getStringFromInferenceTask(inferenceTask, "modelKey")
		if modelKey == "" {
			continue
		}

		// 检查模型是否已存在于管理端
		existingModel, err := s.repoManager.ConvertedModel().FindByModelKey(ctx, modelKey)
		if err == nil && existingModel != nil {
			log.Printf("[BoxClientService] 模型 %s 已存在于管理端", modelKey)
			continue
		}

		// 从map中提取type
		taskType := getStringFromInferenceTask(inferenceTask, "type")

		// 从盒子获取模型详情并同步
		err = s.syncModelFromBox(ctx, box, modelKey, chipType, taskType)
		if err != nil {
			log.Printf("[BoxClientService] 同步模型 %s 失败: %v", modelKey, err)
			// 继续处理其他模型
		}
	}

	return nil
}

// syncModelFromBox 从盒子同步模型
func (s *BoxClientService) syncModelFromBox(ctx context.Context, box *models.Box, modelKey, chipType, taskType string) error {
	// 获取盒子上的模型列表
	modelsResp, err := s.proxyService.GetModels(box.ID)
	if err != nil {
		return fmt.Errorf("获取盒子模型列表失败: %w", err)
	}

	// 查找目标模型 - 使用Name匹配，因为ModelInfo没有ModelKey字段
	// modelKey的格式通常是: type-version-chip-name
	var targetModel *ModelInfo
	for i, model := range modelsResp.Models {
		// 尝试通过名称匹配
		if model.Name == modelKey || strings.Contains(modelKey, model.Name) {
			targetModel = &modelsResp.Models[i]
			break
		}
	}

	// 如果没找到完全匹配，取第一个同类型的模型
	if targetModel == nil && len(modelsResp.Models) > 0 {
		for i, model := range modelsResp.Models {
			if model.Type == taskType {
				targetModel = &modelsResp.Models[i]
				break
			}
		}
	}

	if targetModel == nil {
		log.Printf("[BoxClientService] 盒子上未找到匹配模型: %s，跳过同步", modelKey)
		return nil // 不返回错误，只是记录日志
	}

	// 创建ConvertedModel记录
	convertedModel := &models.ConvertedModel{
		Name:              targetModel.Name,
		DisplayName:       targetModel.Name,
		Description:       fmt.Sprintf("从盒子 %s 同步的模型", box.Name),
		Version:           targetModel.Version,
		ModelKey:          modelKey,
		TaskType:          models.ModelTaskType(taskType),
		TargetChip:        chipType,
		TargetYoloVersion: targetModel.Version,
		Status:            models.ConvertedModelStatusCompleted,
		ConvertedAt:       time.Now(),
		UserID:            0, // 系统同步的模型
	}

	// 保存模型记录
	if err := s.repoManager.ConvertedModel().Create(ctx, convertedModel); err != nil {
		return fmt.Errorf("创建模型记录失败: %w", err)
	}

	log.Printf("[BoxClientService] 成功同步模型: %s", modelKey)
	return nil
}

// getOrCreateVideoSource 获取或创建视频源
func (s *BoxClientService) getOrCreateVideoSource(ctx context.Context, rtspUrl, taskID string) (uint, error) {
	// 尝试根据URL查找现有的视频源
	videoSources, err := s.repoManager.VideoSource().FindByURL(rtspUrl)
	if err == nil && len(videoSources) > 0 {
		return videoSources[0].ID, nil
	}

	// 创建新的视频源
	videoSource := &models.VideoSource{
		Name:        fmt.Sprintf("同步-%s", taskID),
		Description: fmt.Sprintf("从任务 %s 同步的视频源", taskID),
		Type:        models.VideoSourceTypeStream,
		URL:         rtspUrl,
		PlayURL:     rtspUrl,
		Status:      models.VideoSourceStatusActive,
	}

	if err := s.repoManager.VideoSource().Create(videoSource); err != nil {
		return 0, fmt.Errorf("创建视频源失败: %w", err)
	}

	return videoSource.ID, nil
}

// generateUniqueTaskID 生成唯一的任务ID
func (s *BoxClientService) generateUniqueTaskID(ctx context.Context, boxID uint, externalID string) string {
	baseID := fmt.Sprintf("sync_%d_%s", boxID, externalID)

	// 检查是否重复
	if existing, _ := s.repoManager.Task().FindByTaskID(ctx, baseID); existing == nil {
		return baseID
	}

	// 如果重复，添加时间戳
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s_%d", baseID, timestamp)
}

// mapTaskStatus 映射任务状态
func (s *BoxClientService) mapTaskStatus(boxStatus string) models.TaskStatus {
	switch strings.ToUpper(boxStatus) {
	case "RUNNING":
		return models.TaskStatusRunning
	case "STOPPED":
		return models.TaskStatusStopped
	case "STARTING":
		return models.TaskStatusScheduled
	case "STOPPING":
		return models.TaskStatusStopping
	case "ERROR":
		return models.TaskStatusFailed
	default:
		return models.TaskStatusPending
	}
}

// convertTaskConfig 转换任务配置（处理map[string]interface{}类型）
func (s *BoxClientService) convertTaskConfig(taskDetail *TaskDetailResponse, task *models.Task) {
	boxTask := taskDetail.Task

	// 转换输出设置（从map中提取）
	if boxTask.OutputSettings != nil {
		sendFullImage := 0
		if val, ok := boxTask.OutputSettings["sendFullImage"]; ok {
			if fVal, ok := val.(float64); ok {
				sendFullImage = int(fVal)
			} else if iVal, ok := val.(int); ok {
				sendFullImage = iVal
			}
		}
		task.OutputSettings = models.OutputSettings{
			SendFullImage: sendFullImage,
		}
	}

	// 转换ROI配置（从map切片中提取）
	if boxTask.ROIs != nil {
		rois := make([]models.ROIConfig, 0, len(boxTask.ROIs))
		for _, roiMap := range boxTask.ROIs {
			roi := models.ROIConfig{
				ID:     getIntFromMap(roiMap, "id"),
				Name:   getStringFromMap(roiMap, "name"),
				Width:  getIntFromMap(roiMap, "width"),
				Height: getIntFromMap(roiMap, "height"),
				X:      getIntFromMap(roiMap, "x"),
				Y:      getIntFromMap(roiMap, "y"),
			}

			// 处理areas
			if areasVal, ok := roiMap["areas"]; ok {
				if areasSlice, ok := areasVal.([]interface{}); ok {
					areas := make([]models.Point, 0, len(areasSlice))
					for _, areaVal := range areasSlice {
						if areaMap, ok := areaVal.(map[string]interface{}); ok {
							areas = append(areas, models.Point{
								X: getIntFromMap(areaMap, "x"),
								Y: getIntFromMap(areaMap, "y"),
							})
						}
					}
					roi.Areas = areas
				}
			}
			rois = append(rois, roi)
		}
		task.ROIs = rois
	}

	// 转换推理任务配置（从map切片中提取）
	if boxTask.InferenceTasks != nil {
		inferenceTasks := make([]models.InferenceTask, 0, len(boxTask.InferenceTasks))
		for _, itMap := range boxTask.InferenceTasks {
			it := models.InferenceTask{
				Type:            getStringFromMap(itMap, "type"),
				ModelName:       getStringFromMap(itMap, "modelKey"), // 使用modelKey作为ModelName
				Threshold:       getFloat64FromMap(itMap, "threshold"),
				SendSSEImage:    getBoolFromMap(itMap, "sendSSEImage"),
				BusinessProcess: getStringFromMap(itMap, "businessProcess"),
				RtspPushUrl:     getStringFromMap(itMap, "rtspPushUrl"),
			}

			// 处理roiIds
			if roiIdsVal, ok := itMap["roiIds"]; ok {
				if roiIdsSlice, ok := roiIdsVal.([]interface{}); ok {
					roiIds := make([]int, 0, len(roiIdsSlice))
					for _, v := range roiIdsSlice {
						if fVal, ok := v.(float64); ok {
							roiIds = append(roiIds, int(fVal))
						} else if iVal, ok := v.(int); ok {
							roiIds = append(roiIds, iVal)
						}
					}
					it.ROIIds = roiIds
				}
			}

			// 处理forwardInfos
			if forwardInfosVal, ok := itMap["forwardInfos"]; ok {
				if forwardInfosSlice, ok := forwardInfosVal.([]interface{}); ok {
					forwardInfos := make([]models.ForwardInfo, 0, len(forwardInfosSlice))
					for _, fiVal := range forwardInfosSlice {
						if fiMap, ok := fiVal.(map[string]interface{}); ok {
							forwardInfos = append(forwardInfos, models.ForwardInfo{
								Enabled:  getBoolFromMap(fiMap, "enabled"),
								Type:     getStringFromMap(fiMap, "type"),
								Host:     getStringFromMap(fiMap, "host"),
								Port:     getIntFromMap(fiMap, "port"),
								Topic:    getStringFromMap(fiMap, "topic"),
								Username: getStringFromMap(fiMap, "username"),
								Password: getStringFromMap(fiMap, "password"),
							})
						}
					}
					it.ForwardInfos = forwardInfos
				}
			}

			inferenceTasks = append(inferenceTasks, it)
		}
		task.InferenceTasks = inferenceTasks
	}
}

// getBoxChipType 获取盒子的芯片类型
func (s *BoxClientService) getBoxChipType(box *models.Box) string {
	// 从盒子的Meta信息中获取芯片类型
	if len(box.Meta.SupportedHardware) > 0 {
		// 返回第一个支持的硬件类型
		return box.Meta.SupportedHardware[0]
	}

	// 从SDK版本或其他信息推断
	// 默认返回BM1684X
	return "bm1684x"
}

// maskApiKey 掩码API密钥（用于日志）
func maskApiKey(apiKey string) string {
	if apiKey == "" {
		return "<empty>"
	}
	if len(apiKey) <= 8 {
		return "****"
	}
	return apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
}

// 辅助函数：从map中安全获取字符串值
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// 辅助函数：从map中安全获取整数值
func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		if f, ok := val.(float64); ok {
			return int(f)
		}
		if i, ok := val.(int); ok {
			return i
		}
	}
	return 0
}

// 辅助函数：从map中安全获取float64值
func getFloat64FromMap(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
		if i, ok := val.(int); ok {
			return float64(i)
		}
	}
	return 0.0
}

// 辅助函数：从map中安全获取布尔值
func getBoolFromMap(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// 辅助函数：从推理任务map中获取字符串值
func getStringFromInferenceTask(it map[string]interface{}, key string) string {
	return getStringFromMap(it, key)
}

