/*
 * @module service/conversion_service
 * @description 模型转换服务，提供模型转换任务的创建、管理和执行
 * @architecture 服务层
 * @documentReference REQ-003: 模型转换功能
 * @stateFlow Controller -> ConversionService -> Repository -> Database
 * @rules 实现模型转换业务逻辑，包括任务创建、进度跟踪等
 * @dependencies box-manage-service/repository, box-manage-service/models
 * @refs REQ-003.md, DESIGN-005.md, DESIGN-006.md
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// conversionService 模型转换服务实现
type conversionService struct {
	conversionRepo     repository.ConversionTaskRepository
	modelRepo          repository.OriginalModelRepository
	convertedModelRepo repository.ConvertedModelRepository
	dockerService      DockerService
	sseService         SSEService
	config             *ConversionConfig
	logService         SystemLogService // 系统日志服务
}

// NewConversionService 创建模型转换服务实例
func NewConversionService(
	conversionRepo repository.ConversionTaskRepository,
	modelRepo repository.OriginalModelRepository,
	convertedModelRepo repository.ConvertedModelRepository,
	config *ConversionConfig,
	sseService SSEService,
	logService SystemLogService,
) ConversionService {
	return &conversionService{
		conversionRepo:     conversionRepo,
		modelRepo:          modelRepo,
		convertedModelRepo: convertedModelRepo,
		dockerService:      NewDockerService(""), // 使用默认URL
		sseService:         sseService,
		config:             config,
		logService:         logService,
	}
}

// CreateConversionTask 创建转换任务（支持多芯片）
func (s *conversionService) CreateConversionTask(ctx context.Context, req *CreateConversionTaskRequest) ([]*models.ConversionTask, error) {
	log.Printf("[ConversionService] CreateConversionTask started - ModelID: %d, TargetChips: %v, CreatedBy: %d, AutoStart: %t",
		req.OriginalModelID, req.TargetChips, req.CreatedBy, req.AutoStart)

	// 记录任务创建开始日志
	if s.logService != nil {
		s.logService.Info("conversion_service", "转换任务创建开始",
			fmt.Sprintf("开始创建模型转换任务，原始模型ID: %d", req.OriginalModelID),
			WithMetadata(map[string]interface{}{
				"original_model_id": req.OriginalModelID,
				"target_chips":      req.TargetChips,
				"created_by":        req.CreatedBy,
				"auto_start":        req.AutoStart,
			}))
	}

	// 获取原始模型
	originalModel, err := s.modelRepo.GetByID(ctx, req.OriginalModelID)
	if err != nil {
		log.Printf("[ConversionService] Failed to get original model - ModelID: %d, Error: %v", req.OriginalModelID, err)
		return nil, fmt.Errorf("获取原始模型失败: %w", err)
	}

	// 验证模型状态 - 只允许ready状态的模型进行转换
	if originalModel.Status != models.OriginalModelStatusReady {
		log.Printf("[ConversionService] Model status not allowed for conversion - ModelID: %d, Status: %s, Allowed: [ready]",
			originalModel.ID, originalModel.Status)
		return nil, fmt.Errorf("模型状态不允许转换，当前状态: %s，只有ready状态的模型可以转换", originalModel.Status)
	}

	// 验证芯片数量限制
	if len(req.TargetChips) == 0 {
		return nil, errors.New("至少需要选择一个目标芯片")
	}
	if len(req.TargetChips) > 2 {
		return nil, errors.New("最多只能选择2个目标芯片")
	}

	// 验证芯片重复
	chipMap := make(map[string]bool)
	for _, chip := range req.TargetChips {
		if chipMap[chip] {
			return nil, fmt.Errorf("芯片型号重复: %s", chip)
		}
		chipMap[chip] = true
	}

	// 设置默认量化类型
	quantizations := req.Quantizations
	if len(quantizations) == 0 {
		quantizations = []string{"F16"} // 默认量化类型
	}

	// 验证量化类型重复
	quantMap := make(map[string]bool)
	for _, quant := range quantizations {
		if quantMap[quant] {
			return nil, fmt.Errorf("量化类型重复: %s", quant)
		}
		quantMap[quant] = true
	}

	// 验证总任务数限制（chips × quantizations ≤ 4）
	totalTasks := len(req.TargetChips) * len(quantizations)
	if totalTasks > 4 {
		return nil, fmt.Errorf("总任务数超过限制，当前将创建 %d 个任务（%d 个芯片 × %d 个量化类型），最多允许 4 个任务",
			totalTasks, len(req.TargetChips), len(quantizations))
	}

	// 为每个芯片和量化类型的组合创建转换任务
	var tasks []*models.ConversionTask
	for _, chip := range req.TargetChips {
		for _, quantization := range quantizations {
			// 生成任务ID，包含芯片和量化类型信息
			taskID := fmt.Sprintf("conv_%s_%s_%s_%s",
				originalModel.Name, chip, quantization, uuid.New().String()[:8])

			// 从原始模型获取输入参数，设置默认值以防字段为空
			inputChannels := originalModel.InputChannels
			if inputChannels <= 0 {
				inputChannels = 3 // 默认RGB 3通道
			}
			inputHeight := originalModel.InputHeight
			if inputHeight <= 0 {
				inputHeight = 640 // 默认高度
			}
			inputWidth := originalModel.InputWidth
			if inputWidth <= 0 {
				inputWidth = 640 // 默认宽度
			}

			log.Printf("[ConversionService] Using input shape from original model - TaskID: %s, Channels: %d, Height: %d, Width: %d",
				taskID, inputChannels, inputHeight, inputWidth)

			// 设置默认TargetYoloVersion
			targetYoloVersion := req.TargetYoloVersion
			if targetYoloVersion == "" {
				targetYoloVersion = "yolov8" // 默认值
			}

			// 构建转换参数 - 从原始模型获取输入形状
			parameters := models.ConversionParameters{
				TargetChip:        chip,
				TargetYoloVersion: targetYoloVersion,                                // 目标YOLO版本
				InputShape:        []int{1, inputChannels, inputHeight, inputWidth}, // 从原始模型获取
				ModelFormat:       "bmodel",                                         // 默认模型格式
				Quantization:      quantization,                                     // 量化类型
			}

			// 创建转换任务
			task := &models.ConversionTask{
				TaskID:          taskID,
				OriginalModelID: req.OriginalModelID,
				TargetFormat:    "bmodel",
				Parameters:      parameters,
				Status:          models.ConversionTaskStatusPending,
				CreatedBy:       req.CreatedBy,
			}

			// 保存到数据库
			if err := s.conversionRepo.Create(ctx, task); err != nil {
				log.Printf("[ConversionService] Failed to save conversion task - TaskID: %s, Error: %v", taskID, err)
				return nil, fmt.Errorf("保存转换任务失败: %w", err)
			}

			tasks = append(tasks, task)
			log.Printf("[ConversionService] Created conversion task - TaskID: %s, Chip: %s, Quantization: %s", taskID, chip, quantization)

			// 发送任务创建事件
			if s.sseService != nil {
				s.sseService.BroadcastConversionTaskUpdate(task)
			}
		}
	}

	// 如果设置了自动启动，则在所有任务创建完成后统一启动
	if req.AutoStart {
		log.Printf("[ConversionService] Auto-starting %d conversion tasks", len(tasks))

		// 使用独立的 goroutine 来启动任务，避免阻塞响应
		go func() {
			// 使用背景上下文，避免HTTP请求上下文取消影响
			bgCtx := context.Background()

			// 稍微延迟一下，确保数据库事务已提交
			time.Sleep(100 * time.Millisecond)

			for _, task := range tasks {
				log.Printf("[ConversionService] Auto-starting conversion task - TaskID: %s", task.TaskID)
				if err := s.StartConversion(bgCtx, task.TaskID); err != nil {
					log.Printf("[ConversionService] Failed to auto-start conversion - TaskID: %s, Error: %v", task.TaskID, err)

					// 标记任务状态为失败，并记录错误信息
					task.Status = models.ConversionTaskStatusFailed
					task.ErrorMessage = fmt.Sprintf("自动启动失败: %v", err)
					if updateErr := s.conversionRepo.Update(bgCtx, task); updateErr != nil {
						log.Printf("[ConversionService] Failed to update task after auto-start failure - TaskID: %s, UpdateError: %v", task.TaskID, updateErr)
					}
				} else {
					log.Printf("[ConversionService] Auto-start conversion successful - TaskID: %s", task.TaskID)

					// 更新任务对象状态（用于API响应）
					task.Status = models.ConversionTaskStatusRunning
					now := time.Now()
					task.StartTime = &now
				}
			}
		}()
	}

	log.Printf("[ConversionService] CreateConversionTask completed - Created %d tasks", len(tasks))
	return tasks, nil
}

// GetConversionTask 获取转换任务
func (s *conversionService) GetConversionTask(ctx context.Context, taskID string) (*models.ConversionTask, error) {
	return s.conversionRepo.GetByTaskID(ctx, taskID)
}

// GetConversionTasks 获取转换任务列表
func (s *conversionService) GetConversionTasks(ctx context.Context, req *GetConversionTasksRequest) (*GetConversionTasksResponse, error) {
	// 构建查询请求
	repoReq := &repository.GetConversionTaskListRequest{
		Page:            req.Page,
		PageSize:        req.PageSize,
		Status:          req.Status,
		UserID:          req.UserID,
		OriginalModelID: req.OriginalModelID,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		Keyword:         req.Keyword,
	}

	repoResp, err := s.conversionRepo.GetTaskList(ctx, repoReq)
	if err != nil {
		return nil, fmt.Errorf("获取转换任务列表失败: %w", err)
	}

	return &GetConversionTasksResponse{
		Tasks:    repoResp.Tasks,
		Total:    repoResp.Total,
		Page:     repoResp.Page,
		PageSize: repoResp.PageSize,
	}, nil
}

// DeleteConversionTask 删除转换任务
func (s *conversionService) DeleteConversionTask(ctx context.Context, taskID string) error {
	task, err := s.conversionRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("获取转换任务失败: %w", err)
	}

	// 只允许删除已完成、失败的任务
	if task.IsRunning() {
		return errors.New("正在运行的任务不能删除")
	}

	return s.conversionRepo.Delete(ctx, task.ID)
}

// StartConversion 启动转换
func (s *conversionService) StartConversion(ctx context.Context, taskID string) error {
	log.Printf("[ConversionService] StartConversion called - TaskID: %s", taskID)

	// 获取转换任务
	log.Printf("[ConversionService] Fetching conversion task - TaskID: %s", taskID)
	task, err := s.conversionRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		log.Printf("[ConversionService] Failed to get conversion task - TaskID: %s, Error: %v", taskID, err)
		return fmt.Errorf("获取转换任务失败: %w", err)
	}

	log.Printf("[ConversionService] Task found - TaskID: %s, Status: %s, OriginalModelID: %d, TargetChip: %s",
		taskID, task.Status, task.OriginalModelID, task.Parameters.TargetChip)

	// 验证任务状态 - 允许pending和failed状态的任务启动
	if task.Status != models.ConversionTaskStatusPending && task.Status != models.ConversionTaskStatusFailed {
		log.Printf("[ConversionService] Task status not allowed for start - TaskID: %s, Current Status: %s, Allowed: [pending, failed]",
			taskID, task.Status)
		return errors.New("任务状态不允许启动，只有pending和failed状态的任务可以启动")
	}

	// 如果是失败的任务重新启动，重置任务状态
	if task.Status == models.ConversionTaskStatusFailed {
		log.Printf("[ConversionService] Restarting failed task - TaskID: %s", taskID)
		// 清除之前的错误信息
		task.ErrorMessage = ""
		task.Progress = 0
		task.StartTime = nil
		task.EndTime = nil
	}

	// 启动转换
	log.Printf("[ConversionService] Starting conversion task - TaskID: %s", taskID)
	task.Start()

	log.Printf("[ConversionService] Updating task status to running - TaskID: %s", taskID)
	if err := s.conversionRepo.Update(ctx, task); err != nil {
		log.Printf("[ConversionService] Failed to update task status - TaskID: %s, Error: %v", taskID, err)
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 发送任务启动事件
	if s.sseService != nil {
		s.sseService.BroadcastConversionTaskUpdate(task)
	}

	// 异步执行转换 - 使用background context避免HTTP context取消影响
	log.Printf("[ConversionService] Launching async conversion execution - TaskID: %s", taskID)
	go s.executeConversion(context.Background(), task)

	log.Printf("[ConversionService] StartConversion completed successfully - TaskID: %s", taskID)
	return nil
}

// StopConversion 停止转换
func (s *conversionService) StopConversion(ctx context.Context, taskID string) error {
	log.Printf("[ConversionService] StopConversion called - TaskID: %s", taskID)

	// 获取转换任务
	task, err := s.conversionRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		log.Printf("[ConversionService] Failed to get conversion task - TaskID: %s, Error: %v", taskID, err)
		return fmt.Errorf("获取转换任务失败: %w", err)
	}

	log.Printf("[ConversionService] Task found - TaskID: %s, Status: %s", taskID, task.Status)

	// 验证任务状态 - 只有运行中的任务可以停止
	if task.Status != models.ConversionTaskStatusRunning {
		log.Printf("[ConversionService] Task not in running status - TaskID: %s, Current Status: %s", taskID, task.Status)
		return errors.New("只有运行中的任务可以停止")
	}

	// 调用Docker服务停止转换
	log.Printf("[ConversionService] Calling Docker service to stop conversion - TaskID: %s", taskID)
	if err := s.dockerService.StopConversion(ctx, taskID); err != nil {
		log.Printf("[ConversionService] Failed to stop conversion in Docker service - TaskID: %s, Error: %v", taskID, err)
		// 即使Docker服务停止失败，我们也要更新本地状态
	}

	// 更新任务状态为已取消
	log.Printf("[ConversionService] Updating task status to failed (stopped) - TaskID: %s", taskID)
	task.Fail("任务已被用户停止")

	if err := s.conversionRepo.Update(ctx, task); err != nil {
		log.Printf("[ConversionService] Failed to update task status - TaskID: %s, Error: %v", taskID, err)
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 发送任务停止事件
	if s.sseService != nil {
		s.sseService.BroadcastConversionTaskUpdate(task)
	}

	log.Printf("[ConversionService] StopConversion completed successfully - TaskID: %s", taskID)
	return nil
}

// CompleteConversion 完成转换
func (s *conversionService) CompleteConversion(ctx context.Context, taskID string, outputPath string) error {
	task, err := s.conversionRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("获取转换任务失败: %w", err)
	}

	task.Complete(task.OutputPath)
	if outputPath != "" {
		task.OutputPath = outputPath
	}

	// 创建转换后模型记录
	if err := s.createConvertedModel(ctx, task); err != nil {
		return fmt.Errorf("创建转换后模型记录失败: %w", err)
	}

	return s.conversionRepo.Update(ctx, task)
}

// FailConversion 转换失败
func (s *conversionService) FailConversion(ctx context.Context, taskID string, errorMsg string) error {
	task, err := s.conversionRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("获取转换任务失败: %w", err)
	}

	task.Fail(errorMsg)
	return s.conversionRepo.Update(ctx, task)
}

// UpdateConversionProgress 更新转换进度
func (s *conversionService) UpdateConversionProgress(ctx context.Context, taskID string, progress int) error {
	return s.conversionRepo.UpdateProgress(ctx, taskID, progress)
}

// GetConversionProgress 获取转换进度
func (s *conversionService) GetConversionProgress(ctx context.Context, taskID string) (*ConversionProgress, error) {
	task, err := s.conversionRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取转换任务失败: %w", err)
	}

	progress := &ConversionProgress{
		TaskID:          task.TaskID,
		Status:          string(task.Status),
		Progress:        task.Progress,
		ProgressMessage: fmt.Sprintf("进度: %d%%", task.Progress),
		StartTime:       task.StartTime,
		ErrorMessage:    task.ErrorMessage,
	}

	if task.StartTime != nil {
		elapsed := time.Since(*task.StartTime)
		progress.ElapsedTime = elapsed.String()
	}

	return progress, nil
}

// GetConversionLogs 获取转换日志
func (s *conversionService) GetConversionLogs(ctx context.Context, taskID string) ([]string, error) {
	task, err := s.conversionRepo.GetByTaskID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取转换任务失败: %w", err)
	}

	// 返回任务中的日志
	if task.Logs != "" {
		return task.GetLogLines(), nil
	}

	return []string{}, nil
}

// GetConversionStatistics 获取转换统计
func (s *conversionService) GetConversionStatistics(ctx context.Context, userID *uint) (*ConversionStatistics, error) {
	stats, err := s.conversionRepo.GetTaskStatistics(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取转换统计失败: %w", err)
	}

	return &ConversionStatistics{
		TotalTasks:           stats.TotalTasks,
		PendingTasks:         stats.PendingTasks,
		RunningTasks:         stats.RunningTasks,
		CompletedTasks:       stats.CompletedTasks,
		FailedTasks:          stats.FailedTasks,
		SuccessRate:          stats.SuccessRate,
		AverageExecutionTime: stats.AverageExecutionTime.String(),
	}, nil
}

// CleanupFailedTasks 清理失败任务
func (s *conversionService) CleanupFailedTasks(ctx context.Context) (int64, error) {
	olderThan := time.Now().Add(-s.config.FailedTaskRetentionTime)
	return s.conversionRepo.CleanupFailedTasks(ctx, olderThan)
}

// 私有方法

func (s *conversionService) generateOutputPath(model *models.OriginalModel, params *models.ConversionParameters) string {
	fileName := fmt.Sprintf("%s_%s_%s.bmodel",
		strings.ReplaceAll(model.Name, " ", "_"),
		params.TargetChip,
		params.Quantization)
	return filepath.Join("data/models/converted", fileName)
}

// generateUniqueOutputPath 生成唯一的输出文件路径
func (s *conversionService) generateUniqueOutputPath(model *models.OriginalModel, params *models.ConversionParameters, taskID string) string {
	// 使用任务ID的后8位确保文件名唯一
	shortTaskID := taskID
	if len(taskID) > 8 {
		shortTaskID = taskID[len(taskID)-8:]
	}
	fileName := fmt.Sprintf("%s_%s_%s_%s.bmodel",
		strings.ReplaceAll(model.Name, " ", "_"),
		params.TargetChip,
		params.Quantization,
		shortTaskID)
	return filepath.Join("data/models/converted", fileName)
}

func (s *conversionService) createConvertedModel(ctx context.Context, task *models.ConversionTask) error {
	// 获取原始模型信息
	originalModel, err := s.modelRepo.GetByID(ctx, task.OriginalModelID)
	if err != nil {
		return fmt.Errorf("获取原始模型失败: %w", err)
	}

	// 获取文件信息
	fileInfo, err := os.Stat(task.OutputPath)
	if err != nil {
		return fmt.Errorf("获取输出文件信息失败: %w", err)
	}

	// 生成转换后模型名称，包含量化类型
	modelName := fmt.Sprintf("%s_%s_%s",
		originalModel.Name, task.Parameters.TargetChip, task.Parameters.Quantization)

	// 创建转换后模型记录
	convertedModel := &models.ConvertedModel{
		Name:              modelName,
		DisplayName:       fmt.Sprintf("%s (%s, %s, %s)", originalModel.Name, task.Parameters.TargetYoloVersion, task.Parameters.TargetChip, task.Parameters.Quantization),
		Description:       fmt.Sprintf("从%s转换而来", originalModel.Name),
		Version:           "1.0.0",
		OriginalModelID:   task.OriginalModelID,
		ConversionTaskID:  task.TaskID,
		FileName:          filepath.Base(task.OutputPath),
		FilePath:          task.OutputPath,
		ConvertedPath:     task.OutputPath, // 设置转换后文件路径
		FileSize:          fileInfo.Size(),
		TaskType:          originalModel.TaskType,
		InputWidth:        originalModel.InputWidth,
		InputHeight:       originalModel.InputHeight,
		InputChannels:     originalModel.InputChannels,
		ConvertParams:     s.serializeConvertParams(&task.Parameters),
		Quantization:      task.Parameters.Quantization,      // 设置量化类型
		TargetYoloVersion: task.Parameters.TargetYoloVersion, // 设置目标YOLO版本
		TargetChip:        task.Parameters.TargetChip,        // 设置目标芯片
		Status:            models.ConvertedModelStatusCompleted,
		ConvertedAt:       time.Now(),
		UserID:            task.CreatedBy,
	}

	// 设置ModelKey
	convertedModel.SetModelKeyFromParams()

	return s.convertedModelRepo.Create(ctx, convertedModel)
}

func (s *conversionService) serializeInputShape(inputShape []int) string {
	if len(inputShape) == 0 {
		return ""
	}
	var strShape []string
	for _, dim := range inputShape {
		strShape = append(strShape, fmt.Sprintf("%d", dim))
	}
	return strings.Join(strShape, ",")
}

func (s *conversionService) serializeConvertParams(params *models.ConversionParameters) string {
	data := map[string]interface{}{
		"target_chip":   params.TargetChip,
		"input_shape":   params.InputShape,
		"model_format":  params.ModelFormat,
		"custom_params": params.CustomParams,
		"enable_debug":  params.EnableDebug,
		"quantization":  params.Quantization,
	}

	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

func (s *conversionService) executeConversion(ctx context.Context, task *models.ConversionTask) {
	log.Printf("[ConversionService] Starting conversion execution - TaskID: %s", task.TaskID)

	// 获取原始模型
	log.Printf("[ConversionService] Fetching original model - TaskID: %s, OriginalModelID: %d", task.TaskID, task.OriginalModelID)
	originalModel, err := s.modelRepo.GetByID(ctx, task.OriginalModelID)
	if err != nil {
		log.Printf("[ConversionService] Failed to get original model - TaskID: %s, OriginalModelID: %d, Error: %v",
			task.TaskID, task.OriginalModelID, err)
		s.failTask(ctx, task, fmt.Sprintf("获取原始模型失败: %v", err))
		return
	}

	log.Printf("[ConversionService] Original model found - TaskID: %s, ModelName: %s, FilePath: %s, FileSize: %d",
		task.TaskID, originalModel.Name, originalModel.FilePath, originalModel.FileSize)

	// 检查文件是否存在
	if originalModel.FilePath == "" {
		log.Printf("[ConversionService] Original model file path is empty - TaskID: %s, ModelID: %d",
			task.TaskID, originalModel.ID)
		s.failTask(ctx, task, "原始模型文件路径为空")
		return
	}

	// 上传模型到转换服务
	log.Printf("[ConversionService] Uploading model to conversion service - TaskID: %s, FilePath: %s",
		task.TaskID, originalModel.FilePath)
	uploadResult, err := s.dockerService.UploadModel(ctx, task, originalModel.FilePath)
	if err != nil {
		log.Printf("[ConversionService] Failed to upload model - TaskID: %s, FilePath: %s, Error: %v",
			task.TaskID, originalModel.FilePath, err)
		errorMsg := fmt.Sprintf("上传模型失败: %v\n\n详细信息:\n- 原始模型路径: %s\n- 模型文件大小: %d bytes\n- 目标芯片: %s",
			err, originalModel.FilePath, originalModel.FileSize, task.Parameters.TargetChip)
		s.failTask(ctx, task, errorMsg)
		return
	}

	// 解析上传结果：格式为 "modelPath|remoteTaskID"
	parts := strings.Split(uploadResult, "|")
	if len(parts) != 2 {
		log.Printf("[ConversionService] Invalid upload result format - TaskID: %s, Result: %s", task.TaskID, uploadResult)
		s.failTask(ctx, task, "上传结果格式错误")
		return
	}

	modelPath := parts[0]
	remoteTaskID := parts[1]
	log.Printf("[ConversionService] Model uploaded successfully - TaskID: %s, RemotePath: %s, RemoteTaskID: %s",
		task.TaskID, modelPath, remoteTaskID)

	// 启动转换
	log.Printf("[ConversionService] Starting model conversion - TaskID: %s, RemotePath: %s, RemoteTaskID: %s, TargetChip: %s",
		task.TaskID, modelPath, remoteTaskID, task.Parameters.TargetChip)
	if err := s.dockerService.StartConversion(ctx, task, modelPath, remoteTaskID); err != nil {
		log.Printf("[ConversionService] Failed to start conversion - TaskID: %s, Error: %v", task.TaskID, err)
		errorMsg := fmt.Sprintf("启动转换失败: %v\n\n详细信息:\n- 远程模型路径: %s\n- 远程任务ID: %s\n- 目标芯片: %s\n- 输入尺寸: %v",
			err, modelPath, remoteTaskID, task.Parameters.TargetChip, task.Parameters.InputShape)
		s.failTask(ctx, task, errorMsg)
		return
	}

	log.Printf("[ConversionService] Conversion started successfully - TaskID: %s", task.TaskID)

	// 监控转换进度
	log.Printf("[ConversionService] Starting conversion monitoring - TaskID: %s, RemoteTaskID: %s",
		task.TaskID, remoteTaskID)
	s.monitorConversion(ctx, task, remoteTaskID)
}

func (s *conversionService) monitorConversion(ctx context.Context, task *models.ConversionTask, remoteTaskID string) {
	log.Printf("[ConversionService] Starting conversion monitoring - TaskID: %s, RemoteTaskID: %s",
		task.TaskID, remoteTaskID)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	timeout := time.After(10 * time.Minute) // 10分钟超时
	startTime := time.Now()

	log.Printf("[ConversionService] Conversion timeout set to 10 minutes - TaskID: %s, StartTime: %s",
		task.TaskID, startTime.Format("2006-01-02 15:04:05"))

	for {
		select {
		case <-ctx.Done():
			log.Printf("[ConversionService] Conversion cancelled by context - TaskID: %s", task.TaskID)
			s.failTask(ctx, task, "转换被取消")
			return
		case <-timeout:
			elapsed := time.Since(startTime)
			log.Printf("[ConversionService] Conversion timeout reached - TaskID: %s, Elapsed: %s",
				task.TaskID, elapsed.String())

			// 尝试获取最后的状态和日志
			timeoutErrorMsg := fmt.Sprintf("转换超时，耗时: %s", elapsed.String())
			if status, err := s.dockerService.GetConversionStatus(ctx, remoteTaskID); err == nil && status != nil {
				// 如果能获取到状态，包含详细信息
				if len(status.Logs) > 0 {
					timeoutErrorMsg = s.buildDetailedErrorMessage(status)
					timeoutErrorMsg = fmt.Sprintf("转换超时（耗时: %s）\n\n%s", elapsed.String(), timeoutErrorMsg[6:]) // 移除原有的"转换失败："前缀
				}
			}

			s.failTask(ctx, task, timeoutErrorMsg)
			return
		case <-ticker.C:
			// 检查转换状态
			log.Printf("[ConversionService] Checking conversion status - TaskID: %s, RemoteTaskID: %s",
				task.TaskID, remoteTaskID)
			status, err := s.dockerService.GetConversionStatus(ctx, remoteTaskID)
			if err != nil {
				log.Printf("[ConversionService] Failed to get conversion status - TaskID: %s, RemoteTaskID: %s, Error: %v",
					task.TaskID, remoteTaskID, err)
				continue
			}

			// 更新日志
			if len(status.Logs) > 0 {
				s.updateTaskLogs(ctx, task, status.Logs)
			}

			// 更新进度
			if status.Status == "processing" {
				// 估算进度
				progress := s.estimateProgress(status.Logs)
				s.UpdateConversionProgress(ctx, task.TaskID, progress)
			} else if status.Status == "completed" {
				s.completeConversion(ctx, task, status)
				return
			} else if status.Status == "failed" {
				// 构建包含详细日志的错误信息
				errorMsg := s.buildDetailedErrorMessage(status)
				log.Printf("[ConversionService] Conversion failed with detailed logs - TaskID: %s, Error: %s",
					task.TaskID, errorMsg)
				s.failTask(ctx, task, errorMsg)
				return
			}
		}
	}
}

func (s *conversionService) completeConversion(ctx context.Context, task *models.ConversionTask, status *ConversionStatus) {
	log.Printf("[ConversionService] Conversion completed - TaskID: %s, OutputFile: %s", task.TaskID, status.OutputFile)

	// 获取原始模型
	originalModel, err := s.modelRepo.GetByID(ctx, task.OriginalModelID)
	if err != nil {
		s.failTask(ctx, task, fmt.Sprintf("获取原始模型失败: %v", err))
		return
	}

	// 使用Docker服务返回的输出文件路径
	outputPath := status.OutputFile
	if outputPath == "" {
		// 如果Docker服务没有返回输出路径，生成默认路径
		outputPath = s.generateOutputPath(originalModel, &task.Parameters)
		log.Printf("[ConversionService] Using generated output path - TaskID: %s, Path: %s", task.TaskID, outputPath)
	} else {
		log.Printf("[ConversionService] Using Docker service output path - TaskID: %s, Path: %s", task.TaskID, outputPath)
	}

	// 下载转换结果（使用远程任务ID）
	localOutputPath := s.generateUniqueOutputPath(originalModel, &task.Parameters, task.TaskID)

	// 从status中获取远程任务ID进行下载
	remoteTaskID := status.TaskID
	if remoteTaskID == "" {
		log.Printf("[ConversionService] No remote task ID available for download - TaskID: %s, using Docker service path", task.TaskID)
		// 如果没有远程任务ID，直接使用Docker服务提供的路径
		localOutputPath = outputPath
	} else {
		log.Printf("[ConversionService] Attempting to download using remote task ID - TaskID: %s, RemoteTaskID: %s", task.TaskID, remoteTaskID)
		if err := s.dockerService.DownloadResult(ctx, remoteTaskID, localOutputPath); err != nil {
			log.Printf("[ConversionService] Download failed, using Docker service path - TaskID: %s, RemoteTaskID: %s, Error: %v", task.TaskID, remoteTaskID, err)
			// 如果下载失败，使用Docker服务提供的路径作为最终路径
			localOutputPath = outputPath
		} else {
			log.Printf("[ConversionService] Download completed - TaskID: %s, RemoteTaskID: %s, LocalPath: %s", task.TaskID, remoteTaskID, localOutputPath)
		}
	}

	// 更新任务状态为完成
	task.Complete(localOutputPath)
	task.OutputPath = localOutputPath
	if err := s.conversionRepo.Update(ctx, task); err != nil {
		log.Printf("[ConversionService] Failed to update task completion status - TaskID: %s, Error: %v", task.TaskID, err)
	}

	// 发送任务完成事件
	if s.sseService != nil {
		s.sseService.BroadcastConversionTaskUpdate(task)
	}

	// 创建转换后模型记录
	if err := s.createConvertedModel(ctx, task); err != nil {
		log.Printf("[ConversionService] Failed to create converted model record - TaskID: %s, Error: %v", task.TaskID, err)
	} else {
		log.Printf("[ConversionService] Converted model record created successfully - TaskID: %s", task.TaskID)
	}
}

func (s *conversionService) failTask(ctx context.Context, task *models.ConversionTask, errorMsg string) {
	log.Printf("[ConversionService] Conversion failed - TaskID: %s, Error: %s", task.TaskID, errorMsg)
	task.Fail(errorMsg)
	if err := s.conversionRepo.Update(ctx, task); err != nil {
		log.Printf("更新任务失败状态失败: %v", err)
	}

	// 发送任务失败事件
	if s.sseService != nil {
		s.sseService.BroadcastConversionTaskUpdate(task)
	}
}

// updateTaskLogs 更新任务日志
func (s *conversionService) updateTaskLogs(ctx context.Context, task *models.ConversionTask, logs []string) {
	// 将日志数组转换为字符串
	logString := strings.Join(logs, "\n")

	// 如果日志内容有变化，则更新
	if task.Logs != logString {
		task.Logs = logString
		if err := s.conversionRepo.Update(ctx, task); err != nil {
			log.Printf("[ConversionService] Failed to update task logs - TaskID: %s, Error: %v", task.TaskID, err)
		} else {
			log.Printf("[ConversionService] Updated task logs - TaskID: %s, LogLines: %d", task.TaskID, len(logs))
		}
	}
}

func (s *conversionService) estimateProgress(logs []string) int {
	if len(logs) == 0 {
		return 10 // 如果没有日志，返回10%
	}

	// 简单的进度估算：每条日志增加进度
	progress := len(logs) * 10
	if progress > 90 {
		progress = 90 // 最多90%，等待完成
	}
	return progress
}

// 配置结构体
type ConversionConfig struct {
	MaxRetries              int           `yaml:"max_retries"`
	FailedTaskRetentionTime time.Duration `yaml:"failed_task_retention_time"`
	LogPath                 string        `yaml:"log_path"`
	OutputPath              string        `yaml:"output_path"`
	MaxConcurrentTasks      int           `yaml:"max_concurrent_tasks"`
}

// 请求结构体
type CreateConversionTaskRequest struct {
	OriginalModelID   uint     `json:"original_model_id" binding:"required" example:"1"`                             // 原始模型ID
	TargetChips       []string `json:"target_chips" binding:"required" swaggertype:"array,string" example:"BM1684X"` // 目标芯片列表（最多2个）
	TargetYoloVersion string   `json:"target_yolo_version" example:"yolov8"`                                         // 目标YOLO版本，默认yolov8
	CreatedBy         uint     `json:"created_by" binding:"required" example:"1"`                                    // 创建用户ID
	AutoStart         bool     `json:"auto_start" example:"true"`                                                    // 是否自动启动
	Quantizations     []string `json:"quantizations,omitempty" swaggertype:"array,string" example:"F16,F32"`         // 量化类型列表（可选，默认F16）
}

type GetConversionTasksRequest struct {
	Page            int       `json:"page" example:"1"`
	PageSize        int       `json:"page_size" example:"20"`
	Status          string    `json:"status,omitempty" example:"running"`
	UserID          *uint     `json:"user_id,omitempty" example:"1"`
	OriginalModelID *uint     `json:"original_model_id,omitempty" example:"1"`
	StartTime       time.Time `json:"start_time,omitempty" example:"2025-01-01T00:00:00Z"`
	EndTime         time.Time `json:"end_time,omitempty" example:"2025-01-31T23:59:59Z"`
	Keyword         string    `json:"keyword,omitempty" example:"yolo"`
}

type GetConversionTasksResponse struct {
	Tasks    []*models.ConversionTask `json:"tasks"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type ConversionProgress struct {
	TaskID          string     `json:"task_id"`
	Status          string     `json:"status"`
	Progress        int        `json:"progress"`
	ProgressMessage string     `json:"progress_message"`
	StartTime       *time.Time `json:"start_time,omitempty"`
	ElapsedTime     string     `json:"elapsed_time"`
	ErrorMessage    string     `json:"error_message,omitempty"`
}

type ConversionStatistics struct {
	TotalTasks           int64   `json:"total_tasks"`
	PendingTasks         int64   `json:"pending_tasks"`
	RunningTasks         int64   `json:"running_tasks"`
	CompletedTasks       int64   `json:"completed_tasks"`
	FailedTasks          int64   `json:"failed_tasks"`
	SuccessRate          float64 `json:"success_rate"`
	AverageExecutionTime string  `json:"average_execution_time"` // 格式如"25m30s"
}

// RecoverPendingTasks 恢复等待中的转换任务
func (s *conversionService) RecoverPendingTasks(ctx context.Context) error {
	log.Printf("[ConversionService] Starting recovery of pending conversion tasks")

	// 获取所有等待中的任务
	pendingTasks, err := s.conversionRepo.GetPendingTasks(ctx)
	if err != nil {
		log.Printf("[ConversionService] Failed to get pending tasks - Error: %v", err)
		return fmt.Errorf("获取等待任务失败: %w", err)
	}

	if len(pendingTasks) == 0 {
		log.Printf("[ConversionService] No pending tasks found to recover")
		return nil
	}

	log.Printf("[ConversionService] Found %d pending tasks to recover", len(pendingTasks))

	// 逐个启动等待的任务
	for _, task := range pendingTasks {
		log.Printf("[ConversionService] Recovering pending task - TaskID: %s, OriginalModelID: %d, TargetChip: %s",
			task.TaskID, task.OriginalModelID, task.Parameters.TargetChip)

		if err := s.StartConversion(context.Background(), task.TaskID); err != nil {
			log.Printf("[ConversionService] Failed to recover task - TaskID: %s, Error: %v", task.TaskID, err)
			// 继续处理其他任务，不因单个任务失败而停止
			continue
		}

		log.Printf("[ConversionService] Successfully recovered task - TaskID: %s", task.TaskID)
	}

	log.Printf("[ConversionService] Pending tasks recovery completed - Total: %d", len(pendingTasks))
	return nil
}

// CheckRunningTasksTimeout 检查运行中任务的超时情况
func (s *conversionService) CheckRunningTasksTimeout(ctx context.Context) error {
	log.Printf("[ConversionService] Starting timeout check for running conversion tasks")

	// 获取所有运行中的任务
	runningTasks, err := s.conversionRepo.GetRunningTasks(ctx)
	if err != nil {
		log.Printf("[ConversionService] Failed to get running tasks - Error: %v", err)
		return fmt.Errorf("获取运行任务失败: %w", err)
	}

	if len(runningTasks) == 0 {
		log.Printf("[ConversionService] No running tasks found")
		return nil
	}

	log.Printf("[ConversionService] Found %d running tasks to check for timeout", len(runningTasks))

	timeoutThreshold := 10 * time.Minute
	now := time.Now()

	// 检查每个运行中的任务是否超时
	for _, task := range runningTasks {
		if task.StartTime == nil {
			log.Printf("[ConversionService] Task has no start time, skipping - TaskID: %s", task.TaskID)
			continue
		}

		elapsed := now.Sub(*task.StartTime)
		log.Printf("[ConversionService] Checking task timeout - TaskID: %s, StartTime: %s, Elapsed: %s, Timeout: %s",
			task.TaskID, task.StartTime.Format("2006-01-02 15:04:05"), elapsed.String(), timeoutThreshold.String())

		if elapsed > timeoutThreshold {
			log.Printf("[ConversionService] Task timeout detected - TaskID: %s, Elapsed: %s",
				task.TaskID, elapsed.String())

			// 标记任务为超时失败
			errorMsg := fmt.Sprintf("任务启动时超时，运行时间: %s", elapsed.String())
			if err := s.FailConversion(ctx, task.TaskID, errorMsg); err != nil {
				log.Printf("[ConversionService] Failed to mark task as timeout - TaskID: %s, Error: %v",
					task.TaskID, err)
			} else {
				log.Printf("[ConversionService] Task marked as timeout - TaskID: %s", task.TaskID)
			}
		}
	}

	log.Printf("[ConversionService] Running tasks timeout check completed")
	return nil
}

// buildDetailedErrorMessage 构建包含详细日志的错误信息
func (s *conversionService) buildDetailedErrorMessage(status *ConversionStatus) string {
	if status == nil {
		return "转换失败：未知错误"
	}

	var errorMsg strings.Builder

	// 添加主要错误信息
	if status.Error != "" {
		errorMsg.WriteString("转换失败：")
		errorMsg.WriteString(status.Error)
	} else {
		errorMsg.WriteString("转换失败：未知错误")
	}

	// 添加详细日志信息
	if len(status.Logs) > 0 {
		errorMsg.WriteString("\n\n详细日志：\n")

		// 限制日志行数，避免错误信息过长
		maxLogLines := 50
		logLines := status.Logs
		if len(logLines) > maxLogLines {
			// 保留前20行和后30行
			var limitedLogs []string
			limitedLogs = append(limitedLogs, logLines[:20]...)
			limitedLogs = append(limitedLogs, fmt.Sprintf("... (省略 %d 行日志) ...", len(logLines)-maxLogLines))
			limitedLogs = append(limitedLogs, logLines[len(logLines)-30:]...)
			logLines = limitedLogs
		}

		for i, logLine := range logLines {
			if i > 0 {
				errorMsg.WriteString("\n")
			}
			errorMsg.WriteString(logLine)
		}
	}

	// 添加任务相关信息
	if status.TaskID != "" {
		errorMsg.WriteString(fmt.Sprintf("\n\n远程任务ID: %s", status.TaskID))
	}
	if status.ModelType != "" {
		errorMsg.WriteString(fmt.Sprintf("\n模型类型: %s", status.ModelType))
	}
	if status.InputShape != "" {
		errorMsg.WriteString(fmt.Sprintf("\n输入尺寸: %s", status.InputShape))
	}

	return errorMsg.String()
}
