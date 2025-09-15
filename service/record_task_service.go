package service

import (
	"box-manage-service/client"
	"box-manage-service/config"
	"box-manage-service/models"
	"box-manage-service/modules/ffmpeg"
	"box-manage-service/repository"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RecordTaskService 录制任务服务实现
type recordTaskService struct {
	taskRepo        repository.RecordTaskRepository
	videoSourceRepo repository.VideoSourceRepository
	ffmpegModule    *ffmpeg.FFmpegModule
	zlmClient       *client.ZLMediaKitClient
	basePath        string
	zlmConfig       config.VideoConfig // 添加ZLMediaKit配置
	sseService      SSEService
}

// NewRecordTaskService 创建新的录制任务服务
func NewRecordTaskService(
	taskRepo repository.RecordTaskRepository,
	videoSourceRepo repository.VideoSourceRepository,
	ffmpegModule *ffmpeg.FFmpegModule,
	zlmClient *client.ZLMediaKitClient,
	basePath string,
	zlmConfig config.VideoConfig,
	sseService SSEService,
) RecordTaskService {
	return &recordTaskService{
		taskRepo:        taskRepo,
		videoSourceRepo: videoSourceRepo,
		ffmpegModule:    ffmpegModule,
		zlmClient:       zlmClient,
		basePath:        basePath,
		zlmConfig:       zlmConfig,
		sseService:      sseService,
	}
}

// CreateRecordTask 创建录制任务
func (s *recordTaskService) CreateRecordTask(ctx context.Context, req *CreateRecordTaskRequest) (*models.RecordTask, error) {
	log.Printf("[DEBUG] CreateRecordTask started - UserID: %d, VideoSourceID: %d, Duration: %d, Name: %s",
		req.UserID, req.VideoSourceID, req.Duration, req.Name)

	// 生成任务ID
	taskID := fmt.Sprintf("record_%d_%d", time.Now().Unix(), req.UserID)
	log.Printf("[DEBUG] Generated taskID: %s", taskID)

	// 验证视频源
	log.Printf("[DEBUG] Validating video source with ID: %d", req.VideoSourceID)
	vs, err := s.videoSourceRepo.GetByID(req.VideoSourceID)
	if err != nil {
		log.Printf("[ERROR] Failed to get video source %d: %v", req.VideoSourceID, err)
		return nil, fmt.Errorf("视频源不存在: %w", err)
	}
	log.Printf("[DEBUG] Video source found - ID: %d, Status: %s, URL: %s", vs.ID, vs.Status, vs.URL)

	// 检查视频源状态
	if vs.Status != models.VideoSourceStatusActive {
		log.Printf("[ERROR] Video source %d is not active, current status: %s", vs.ID, vs.Status)
		return nil, fmt.Errorf("视频源未激活")
	}

	// 创建任务对象
	task := &models.RecordTask{
		TaskID:        taskID,
		Name:          req.Name,
		Duration:      req.Duration,
		Status:        models.RecordTaskStatusPending,
		VideoSourceID: req.VideoSourceID,
		UserID:        req.UserID,
	}
	log.Printf("[DEBUG] Created task object: %+v", task)

	// 保存任务
	log.Printf("[DEBUG] Saving record task to database")
	if err := s.taskRepo.Create(task); err != nil {
		log.Printf("[ERROR] Failed to create record task in database: %v", err)
		return nil, fmt.Errorf("创建录制任务失败: %w", err)
	}
	log.Printf("[DEBUG] Record task saved successfully with ID: %d", task.ID)

	// 发送录制任务创建事件
	if s.sseService != nil {
		metadata := map[string]interface{}{
			"task_id":         task.ID,
			"task_name":       task.Name,
			"user_id":         task.UserID,
			"video_source_id": task.VideoSourceID,
			"duration":        task.Duration,
			"source":          "record_task_service",
			"source_id":       fmt.Sprintf("record_task_%d", task.ID),
			"title":           "录制任务已创建",
			"message":         fmt.Sprintf("录制任务 %s 已创建", task.Name),
		}
		// 创建一个临时的Task结构用于事件广播
		s.sseService.BroadcastRecordTaskCreated(task, metadata)
	}

	// 自动开始任务
	log.Printf("[DEBUG] Auto-starting record task %d", task.ID)
	if err := s.StartRecordTask(ctx, task.ID); err != nil {
		// 如果开始失败，记录错误但不阻止任务创建
		log.Printf("[WARNING] Failed to auto-start record task %d: %v", task.ID, err)
		fmt.Printf("Warning: Failed to auto-start record task %d: %v\n", task.ID, err)
	} else {
		log.Printf("[DEBUG] Record task %d auto-started successfully", task.ID)
	}

	log.Printf("[DEBUG] CreateRecordTask completed successfully - TaskID: %s, DatabaseID: %d", taskID, task.ID)
	return task, nil
}

// GetRecordTask 获取录制任务
func (s *recordTaskService) GetRecordTask(ctx context.Context, id uint) (*models.RecordTask, error) {
	log.Printf("[DEBUG] GetRecordTask called for task ID: %d", id)

	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		log.Printf("[ERROR] Failed to get record task %d: %v", id, err)
		return nil, err
	}

	if task != nil {
		log.Printf("[DEBUG] Found record task - ID: %d, TaskID: %s, Status: %s, Duration: %d",
			task.ID, task.TaskID, task.Status, task.Duration)
	} else {
		log.Printf("[DEBUG] No record task found with ID: %d", id)
	}

	return task, nil
}

// GetRecordTasks 获取录制任务列表
func (s *recordTaskService) GetRecordTasks(ctx context.Context, req *GetRecordTasksRequest) ([]*models.RecordTask, int64, error) {
	log.Printf("[DEBUG] GetRecordTasks called - Page: %d, PageSize: %d, Status: %v, VideoSourceID: %v",
		req.Page, req.PageSize, req.Status, req.VideoSourceID)

	// 分页参数
	page := 1
	pageSize := 20
	if req.Page > 0 {
		page = req.Page
	}
	if req.PageSize > 0 {
		pageSize = req.PageSize
	}
	log.Printf("[DEBUG] Final pagination parameters - Page: %d, PageSize: %d", page, pageSize)

	// 转换状态参数
	var status *models.RecordTaskStatus
	if req.Status != nil {
		s := models.RecordTaskStatus(*req.Status)
		status = &s
		log.Printf("[DEBUG] Status filter applied: %s", s)
	} else {
		log.Printf("[DEBUG] No status filter applied")
	}

	tasks, total, err := s.taskRepo.List(page, pageSize, status, req.VideoSourceID)
	if err != nil {
		log.Printf("[ERROR] Failed to get record tasks list: %v", err)
		return nil, 0, err
	}

	log.Printf("[DEBUG] Retrieved %d tasks out of %d total", len(tasks), total)
	return tasks, total, nil
}

// StartRecordTask 启动录制任务
func (s *recordTaskService) StartRecordTask(ctx context.Context, id uint) error {
	log.Printf("[DEBUG] StartRecordTask called for task ID: %d", id)

	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		log.Printf("[ERROR] Failed to get record task %d for starting: %v", id, err)
		return fmt.Errorf("获取录制任务失败: %w", err)
	}
	log.Printf("[DEBUG] Retrieved task for starting - ID: %d, TaskID: %s, Status: %s", task.ID, task.TaskID, task.Status)

	if !task.CanStart() {
		log.Printf("[ERROR] Task %d cannot be started, current status: %s", id, task.Status)
		return fmt.Errorf("任务状态不允许启动: %s", task.Status)
	}

	// 获取视频源
	log.Printf("[DEBUG] Getting video source %d for task %d", task.VideoSourceID, id)
	vs, err := s.videoSourceRepo.GetByID(task.VideoSourceID)
	if err != nil {
		log.Printf("[ERROR] Failed to get video source %d for task %d: %v", task.VideoSourceID, id, err)
		return fmt.Errorf("获取视频源失败: %w", err)
	}
	log.Printf("[DEBUG] Video source retrieved - ID: %d, Status: %s, URL: %s, PlayURL: %s",
		vs.ID, vs.Status, vs.URL, vs.PlayURL)

	// 检查视频源状态
	if vs.Status != models.VideoSourceStatusActive {
		log.Printf("[ERROR] Video source %d is not active for task %d, status: %s", vs.ID, id, vs.Status)
		return fmt.Errorf("视频源未激活")
	}

	// 标记任务为录制中
	log.Printf("[DEBUG] Marking task %d as recording", id)
	task.MarkAsRecording()
	if err := s.taskRepo.Update(task); err != nil {
		log.Printf("[ERROR] Failed to update task %d status to recording: %v", id, err)
		return fmt.Errorf("更新任务状态失败: %w", err)
	}
	log.Printf("[DEBUG] Task %d status updated to recording successfully", id)

	// 发送录制任务开始事件
	if s.sseService != nil {
		metadata := map[string]interface{}{
			"task_id":         task.ID,
			"task_name":       task.Name,
			"user_id":         task.UserID,
			"video_source_id": task.VideoSourceID,
			"duration":        task.Duration,
			"status":          task.Status,
			"source":          "record_task_service",
			"source_id":       fmt.Sprintf("record_task_%d", task.ID),
			"title":           "录制任务已开始",
			"message":         fmt.Sprintf("录制任务 %d 已开始执行", task.ID),
		}
		s.sseService.BroadcastRecordTaskStarted(task.ID, metadata)
	}

	// 异步执行录制
	log.Printf("[DEBUG] Starting async record execution for task %d", id)
	go func() {
		log.Printf("[DEBUG] Async record goroutine started for task %d", id)
		if err := s.executeRecordTask(context.Background(), task, vs); err != nil {
			log.Printf("[ERROR] Record execution failed for task %d: %v", id, err)
			task.MarkAsFailed(err.Error())
			if updateErr := s.taskRepo.Update(task); updateErr != nil {
				log.Printf("[ERROR] Failed to update task %d status to failed: %v", id, updateErr)
			} else {
				log.Printf("[DEBUG] Task %d marked as failed due to execution error", id)
			}

			// 发送录制任务失败事件
			if s.sseService != nil {
				metadata := map[string]interface{}{
					"task_id":         task.ID,
					"task_name":       task.Name,
					"user_id":         task.UserID,
					"video_source_id": task.VideoSourceID,
					"error":           err.Error(),
					"source":          "record_task_service",
					"source_id":       fmt.Sprintf("record_task_%d", task.ID),
					"title":           "录制任务失败",
					"message":         fmt.Sprintf("录制任务 %d 执行失败: %v", task.ID, err),
				}
				s.sseService.BroadcastRecordTaskFailed(task.ID, err.Error(), metadata)
			}
		} else {
			log.Printf("[DEBUG] Record execution completed successfully for task %d", id)
		}
	}()

	log.Printf("[DEBUG] StartRecordTask completed successfully for task ID: %d", id)
	return nil
}

// StopRecordTask 停止录制任务
func (s *recordTaskService) StopRecordTask(ctx context.Context, id uint) error {
	log.Printf("[DEBUG] StopRecordTask called for task ID: %d", id)

	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		log.Printf("[ERROR] Failed to get record task %d for stopping: %v", id, err)
		return fmt.Errorf("获取录制任务失败: %w", err)
	}
	log.Printf("[DEBUG] Retrieved task for stopping - ID: %d, TaskID: %s, Status: %s", task.ID, task.TaskID, task.Status)

	if !task.CanStop() {
		log.Printf("[ERROR] Task %d cannot be stopped, current status: %s", id, task.Status)
		return fmt.Errorf("任务状态不允许停止: %s", task.Status)
	}

	// 标记任务为已停止
	log.Printf("[DEBUG] Marking task %d as stopped", id)
	task.MarkAsStopped()

	if err := s.taskRepo.Update(task); err != nil {
		log.Printf("[ERROR] Failed to update task %d status to stopped: %v", id, err)
		return err
	}

	log.Printf("[DEBUG] Task %d stopped successfully", id)
	return nil
}

// DeleteRecordTask 删除录制任务
func (s *recordTaskService) DeleteRecordTask(ctx context.Context, id uint) error {
	log.Printf("[DEBUG] DeleteRecordTask called for task ID: %d", id)

	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		log.Printf("[ERROR] Failed to get record task %d for deletion: %v", id, err)
		return fmt.Errorf("获取录制任务失败: %w", err)
	}
	log.Printf("[DEBUG] Retrieved task for deletion - ID: %d, TaskID: %s, Status: %s, OutputPath: %s",
		task.ID, task.TaskID, task.Status, task.OutputPath)

	// 检查任务是否可以删除
	if task.IsActive() {
		log.Printf("[ERROR] Cannot delete active task %d, status: %s", id, task.Status)
		return fmt.Errorf("无法删除正在执行的任务")
	}

	// 删除录制文件
	if task.OutputPath != "" {
		log.Printf("[DEBUG] Attempting to delete output file: %s", task.OutputPath)
		if err := os.Remove(task.OutputPath); err != nil {
			log.Printf("[WARNING] Failed to delete output file %s: %v", task.OutputPath, err)
		} else {
			log.Printf("[DEBUG] Output file deleted successfully: %s", task.OutputPath)
		}

		// 删除目录（如果为空）
		outputDir := filepath.Dir(task.OutputPath)
		log.Printf("[DEBUG] Attempting to delete output directory if empty: %s", outputDir)
		if err := os.Remove(outputDir); err != nil {
			log.Printf("[DEBUG] Output directory not deleted (may not be empty): %s, error: %v", outputDir, err)
		} else {
			log.Printf("[DEBUG] Output directory deleted successfully: %s", outputDir)
		}
	} else {
		log.Printf("[DEBUG] No output file to delete for task %d", id)
	}

	// 删除任务
	log.Printf("[DEBUG] Deleting task %d from database", id)
	if err := s.taskRepo.Delete(id); err != nil {
		log.Printf("[ERROR] Failed to delete task %d from database: %v", id, err)
		return err
	}

	log.Printf("[DEBUG] Task %d deleted successfully", id)
	return nil
}

// DownloadRecord 下载录制文件
func (s *recordTaskService) DownloadRecord(ctx context.Context, id uint) (*DownloadInfo, error) {
	log.Printf("[DEBUG] DownloadRecord called for task ID: %d", id)

	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		log.Printf("[ERROR] Failed to get record task %d for download: %v", id, err)
		return nil, fmt.Errorf("获取录制任务失败: %w", err)
	}
	log.Printf("[DEBUG] Retrieved task for download - ID: %d, TaskID: %s, Status: %s, OutputPath: %s, FileSize: %d",
		task.ID, task.TaskID, task.Status, task.OutputPath, task.FileSize)

	if task.Status != models.RecordTaskStatusCompleted {
		log.Printf("[ERROR] Task %d is not completed, current status: %s", id, task.Status)
		return nil, fmt.Errorf("录制任务未完成")
	}

	if task.OutputPath == "" {
		log.Printf("[ERROR] Task %d has no output path", id)
		return nil, fmt.Errorf("录制文件不存在")
	}

	// 检查文件是否存在
	log.Printf("[DEBUG] Checking if output file exists: %s", task.OutputPath)
	fileInfo, err := os.Stat(task.OutputPath)
	if os.IsNotExist(err) {
		log.Printf("[ERROR] Output file does not exist: %s", task.OutputPath)
		return nil, fmt.Errorf("录制文件不存在")
	}
	if err != nil {
		log.Printf("[ERROR] Failed to stat output file %s: %v", task.OutputPath, err)
		return nil, fmt.Errorf("检查录制文件失败: %w", err)
	}

	actualFileSize := fileInfo.Size()
	log.Printf("[DEBUG] File exists - Path: %s, Size: %d bytes, Database FileSize: %d bytes",
		task.OutputPath, actualFileSize, task.FileSize)

	downloadInfo := &DownloadInfo{
		FilePath:    task.OutputPath,
		FileName:    filepath.Base(task.OutputPath),
		FileSize:    task.FileSize,
		ContentType: "video/mp4",
	}

	log.Printf("[DEBUG] DownloadRecord prepared - FileName: %s, FileSize: %d, ContentType: %s",
		downloadInfo.FileName, downloadInfo.FileSize, downloadInfo.ContentType)

	return downloadInfo, nil
}

// executeRecordTask 执行录制任务
func (s *recordTaskService) executeRecordTask(ctx context.Context, task *models.RecordTask, vs *models.VideoSource) error {
	startTime := time.Now()
	log.Printf("[DEBUG] executeRecordTask started - TaskID: %s, VideoSourceID: %d, Duration: %d minutes",
		task.TaskID, task.VideoSourceID, task.Duration)

	// 确定输入URL
	inputURL := vs.PlayURL
	if inputURL == "" {
		inputURL = vs.URL // 如果没有播放地址，使用原始URL
		log.Printf("[DEBUG] Using original URL as PlayURL is empty - URL: %s", inputURL)
	} else {
		log.Printf("[DEBUG] Using PlayURL - URL: %s", inputURL)
	}

	// 处理相对URL，如果是相对地址，添加ZLMediaKit的host和port前缀
	finalInputURL, err := s.resolveURL(inputURL)
	if err != nil {
		log.Printf("[ERROR] Failed to resolve input URL %s: %v", inputURL, err)
		return fmt.Errorf("解析输入URL失败: %w", err)
	}
	if finalInputURL != inputURL {
		log.Printf("[DEBUG] Resolved relative URL - Original: %s, Final: %s", inputURL, finalInputURL)
	}

	// 创建输出目录
	outputDir := filepath.Join(s.basePath, "records", task.TaskID)
	log.Printf("[DEBUG] Creating output directory: %s", outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("[ERROR] Failed to create output directory %s: %v", outputDir, err)
		return fmt.Errorf("创建输出目录失败: %w", err)
	}
	log.Printf("[DEBUG] Output directory created successfully: %s", outputDir)

	// 生成输出文件路径
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.mp4", task.TaskID))
	log.Printf("[DEBUG] Output file path: %s", outputPath)

	// 设置超时上下文
	timeoutDuration := time.Duration(task.Duration+30) * time.Second
	recordCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()
	log.Printf("[DEBUG] Record timeout set to %v (duration: %d minutes + 30 seconds)", timeoutDuration, task.Duration)

	// 配置录制参数
	recordReq := &ffmpeg.RecordStreamRequest{
		StreamURL:  finalInputURL, // 使用解析后的URL
		OutputPath: outputPath,
		Duration:   task.Duration * 60, // 转换为秒
		Format:     "mp4",
		VideoCodec: "h264",
		AudioCodec: "aac",
	}
	log.Printf("[DEBUG] FFmpeg record parameters - StreamURL: %s, OutputPath: %s, Duration: %d seconds, Format: %s, VideoCodec: %s, AudioCodec: %s",
		recordReq.StreamURL, recordReq.OutputPath, recordReq.Duration, recordReq.Format, recordReq.VideoCodec, recordReq.AudioCodec)

	// 执行录制
	log.Printf("[DEBUG] Starting FFmpeg record stream for task %s", task.TaskID)
	recordStartTime := time.Now()
	if err := s.ffmpegModule.RecordStream(recordCtx, recordReq); err != nil {
		recordDuration := time.Since(recordStartTime)
		log.Printf("[ERROR] FFmpeg record failed for task %s after %v: %v", task.TaskID, recordDuration, err)
		return fmt.Errorf("录制失败: %w", err)
	}
	recordDuration := time.Since(recordStartTime)
	log.Printf("[DEBUG] FFmpeg record completed for task %s, duration: %v", task.TaskID, recordDuration)

	// 获取录制文件信息
	log.Printf("[DEBUG] Getting output file info: %s", outputPath)
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		log.Printf("[ERROR] Failed to get output file info %s: %v", outputPath, err)
		return fmt.Errorf("录制文件不存在: %w", err)
	}

	fileSize := fileInfo.Size()
	log.Printf("[DEBUG] Output file info - Size: %d bytes, ModTime: %v", fileSize, fileInfo.ModTime())

	// 计算实际录制时长
	actualDuration := float64(task.Duration * 60) // 暂时使用配置的时长
	log.Printf("[DEBUG] Calculated actual duration: %.2f seconds", actualDuration)

	// 标记任务完成
	log.Printf("[DEBUG] Marking task %s as completed - OutputPath: %s, FileSize: %d, Duration: %.2f",
		task.TaskID, outputPath, fileSize, actualDuration)
	task.MarkAsCompleted(outputPath, fileSize, actualDuration)

	if err := s.taskRepo.Update(task); err != nil {
		log.Printf("[ERROR] Failed to update task %s as completed: %v", task.TaskID, err)
		return err
	}

	// 发送录制任务完成事件
	if s.sseService != nil {
		metadata := map[string]interface{}{
			"task_id":         task.ID,
			"task_name":       task.Name,
			"user_id":         task.UserID,
			"video_source_id": task.VideoSourceID,
			"output_path":     outputPath,
			"file_size":       fileSize,
			"duration":        actualDuration,
			"source":          "record_task_service",
			"source_id":       fmt.Sprintf("record_task_%d", task.ID),
			"title":           "录制任务已完成",
			"message":         fmt.Sprintf("录制任务 %d 已成功完成，文件大小: %.2f MB", task.ID, float64(fileSize)/(1024*1024)),
		}
		s.sseService.BroadcastRecordTaskCompleted(task.ID, fmt.Sprintf("录制完成，文件大小: %.2f MB", float64(fileSize)/(1024*1024)), metadata)
	}

	totalDuration := time.Since(startTime)
	log.Printf("[DEBUG] executeRecordTask completed successfully for task %s, total duration: %v", task.TaskID, totalDuration)
	return nil
}

// GetTaskStatistics 获取任务统计信息
func (s *recordTaskService) GetTaskStatistics(ctx context.Context, userID *uint) (*RecordTaskStatistics, error) {
	log.Printf("[DEBUG] GetTaskStatistics called - UserID: %v", userID)

	var stats RecordTaskStatistics
	startTime := time.Now()

	// 统计各状态的任务数量
	for _, status := range []models.RecordTaskStatus{
		models.RecordTaskStatusPending,
		models.RecordTaskStatusRecording,
		models.RecordTaskStatusCompleted,
		models.RecordTaskStatusStopped,
		models.RecordTaskStatusFailed,
	} {
		log.Printf("[DEBUG] Counting tasks with status: %s", status)
		count, err := s.taskRepo.Count(&status, userID)
		if err != nil {
			log.Printf("[ERROR] Failed to count tasks with status %s: %v", status, err)
			return nil, fmt.Errorf("统计任务数量失败: %w", err)
		}
		log.Printf("[DEBUG] Count for status %s: %d", status, count)

		switch status {
		case models.RecordTaskStatusPending:
			stats.PendingCount = int(count)
		case models.RecordTaskStatusRecording:
			stats.RecordingCount = int(count)
		case models.RecordTaskStatusCompleted:
			stats.CompletedCount = int(count)
		case models.RecordTaskStatusStopped:
			stats.StoppedCount = int(count)
		case models.RecordTaskStatusFailed:
			stats.FailedCount = int(count)
		}
	}

	stats.TotalCount = stats.PendingCount + stats.RecordingCount + stats.CompletedCount + stats.StoppedCount + stats.FailedCount

	duration := time.Since(startTime)
	log.Printf("[DEBUG] GetTaskStatistics completed - Total: %d, Pending: %d, Recording: %d, Completed: %d, Stopped: %d, Failed: %d, Duration: %v",
		stats.TotalCount, stats.PendingCount, stats.RecordingCount, stats.CompletedCount, stats.StoppedCount, stats.FailedCount, duration)

	return &stats, nil
}

// resolveURL 解析URL，如果是相对地址则添加ZLMediaKit的前缀
func (s *recordTaskService) resolveURL(inputURL string) (string, error) {
	if inputURL == "" {
		return "", fmt.Errorf("输入URL不能为空")
	}

	log.Printf("[DEBUG] resolveURL called with input: %s", inputURL)

	// 解析URL
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		log.Printf("[ERROR] Failed to parse URL %s: %v", inputURL, err)
		return "", fmt.Errorf("URL格式错误: %w", err)
	}

	// 如果已经是完整的URL（有scheme），直接返回
	if parsedURL.Scheme != "" {
		log.Printf("[DEBUG] URL is already absolute: %s", inputURL)
		return inputURL, nil
	}

	// 如果是相对URL，构建完整的URL
	zlmHost := s.zlmConfig.ZLMediaKit.Host
	zlmPort := s.zlmConfig.ZLMediaKit.Port

	// 处理路径，去除路由前缀
	path := parsedURL.Path
	routePrefix := s.zlmConfig.PlayURL.RoutePrefix

	// 确保路径以/开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 去除路由前缀（如果存在）
	if routePrefix != "" && strings.HasPrefix(path, routePrefix) {
		path = strings.TrimPrefix(path, routePrefix)
		log.Printf("[DEBUG] Removed route prefix '%s' from path, new path: %s", routePrefix, path)

		// 确保去除前缀后路径仍以/开头
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	// 保留查询参数和片段
	resolvedURL := fmt.Sprintf("http://%s:%d%s", zlmHost, zlmPort, path)
	if parsedURL.RawQuery != "" {
		resolvedURL += "?" + parsedURL.RawQuery
	}
	if parsedURL.Fragment != "" {
		resolvedURL += "#" + parsedURL.Fragment
	}

	log.Printf("[DEBUG] Resolved relative URL - ZLMHost: %s, ZLMPort: %d, RoutePrefix: %s, OriginalPath: %s, FinalPath: %s, Final: %s",
		zlmHost, zlmPort, routePrefix, parsedURL.Path, path, resolvedURL)

	return resolvedURL, nil
}
