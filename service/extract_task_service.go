package service

import (
	"archive/zip"
	"box-manage-service/models"
	"box-manage-service/modules/ffmpeg"
	"box-manage-service/repository"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExtractTaskService 抽帧任务服务实现
type extractTaskService struct {
	taskRepo        repository.ExtractTaskRepository
	frameRepo       repository.ExtractFrameRepository
	videoSourceRepo repository.VideoSourceRepository
	videoFileRepo   repository.VideoFileRepository
	ffmpegModule    *ffmpeg.FFmpegModule
	basePath        string // 抽帧输出基础路径
	videoBasePath   string // 视频文件基础路径，用于解析视频文件输入路径
	sseService      SSEService
}

// NewExtractTaskService 创建新的抽帧任务服务
// basePath: 抽帧输出基础路径（存储抽帧结果）
// videoBasePath: 视频文件基础路径（用于解析视频文件输入路径）
func NewExtractTaskService(
	taskRepo repository.ExtractTaskRepository,
	frameRepo repository.ExtractFrameRepository,
	videoSourceRepo repository.VideoSourceRepository,
	videoFileRepo repository.VideoFileRepository,
	ffmpegModule *ffmpeg.FFmpegModule,
	basePath string,
	videoBasePath string,
	sseService SSEService,
) ExtractTaskService {
	return &extractTaskService{
		taskRepo:        taskRepo,
		frameRepo:       frameRepo,
		videoSourceRepo: videoSourceRepo,
		videoFileRepo:   videoFileRepo,
		ffmpegModule:    ffmpegModule,
		basePath:        basePath,
		videoBasePath:   videoBasePath,
		sseService:      sseService,
	}
}

// CreateExtractTask 创建抽帧任务
func (s *extractTaskService) CreateExtractTask(ctx context.Context, req *CreateExtractTaskRequest) (*models.ExtractTask, error) {
	// 生成任务ID
	taskID := fmt.Sprintf("extract_%d_%d", time.Now().Unix(), req.UserID)

	// 创建任务对象
	task := &models.ExtractTask{
		TaskID:      taskID,
		Name:        req.Name,
		Description: req.Description,
		FrameCount:  req.FrameCount,
		Duration:    req.Duration,
		Interval:    req.Interval,
		StartTime:   req.StartTime,
		Quality:     req.Quality,
		Format:      req.Format,
		Status:      models.ExtractTaskStatusPending,
		UserID:      req.UserID,
	}

	// 设置数据源
	if req.VideoSourceID != nil {
		task.VideoSourceID = req.VideoSourceID
	}
	if req.VideoFileID != nil {
		task.VideoFileID = req.VideoFileID
	}

	// 验证任务配置
	if err := task.Validate(); err != nil {
		return nil, fmt.Errorf("任务配置验证失败: %w", err)
	}

	// 验证数据源是否存在
	if err := s.validateDataSource(ctx, task); err != nil {
		return nil, err
	}

	// 保存任务
	if err := s.taskRepo.Create(task); err != nil {
		return nil, fmt.Errorf("创建抽帧任务失败: %w", err)
	}

	// 发送抽帧任务创建事件
	if s.sseService != nil {
		metadata := map[string]interface{}{
			"task_id":         task.ID,
			"task_name":       task.Name,
			"user_id":         task.UserID,
			"video_source_id": task.VideoSourceID,
			"video_file_id":   task.VideoFileID,
			"frame_count":     task.FrameCount,
			"duration":        task.Duration,
			"source":          "extract_task_service",
			"source_id":       fmt.Sprintf("extract_task_%d", task.ID),
			"title":           "抽帧任务已创建",
			"message":         fmt.Sprintf("抽帧任务 %s 已创建", task.Name),
		}
		// 创建一个临时的Task结构用于事件广播
		s.sseService.BroadcastExtractTaskCreated(task, metadata)
	}

	// 自动开始任务
	if err := s.StartExtractTask(ctx, task.ID); err != nil {
		// 如果开始失败，记录错误但不阻止任务创建
		fmt.Printf("Warning: Failed to auto-start extract task %d: %v\n", task.ID, err)
	}

	return task, nil
}

// GetExtractTask 获取抽帧任务
func (s *extractTaskService) GetExtractTask(ctx context.Context, id uint) (*models.ExtractTask, error) {
	return s.taskRepo.GetByID(id)
}

// GetExtractTasks 获取抽帧任务列表
func (s *extractTaskService) GetExtractTasks(ctx context.Context, req *GetExtractTasksRequest) ([]*models.ExtractTask, int64, error) {
	// 构建查询条件
	query := make(map[string]interface{})

	if req.Status != nil {
		query["status"] = *req.Status
	}
	if req.VideoSourceID != nil {
		query["video_source_id"] = *req.VideoSourceID
	}
	if req.VideoFileID != nil {
		query["video_file_id"] = *req.VideoFileID
	}
	if req.UserID != nil {
		query["user_id"] = *req.UserID
	}

	// 分页参数
	offset := 0
	limit := 20
	if req.Page > 0 && req.PageSize > 0 {
		offset = (req.Page - 1) * req.PageSize
		limit = req.PageSize
	}

	return s.taskRepo.List(query, offset, limit)
}

// StartExtractTask 启动抽帧任务
func (s *extractTaskService) StartExtractTask(ctx context.Context, id uint) error {
	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("获取抽帧任务失败: %w", err)
	}

	if !task.CanStart() {
		return fmt.Errorf("任务状态不允许启动: %s", task.Status)
	}

	// 标记任务为执行中
	task.MarkAsExtracting()
	if err := s.taskRepo.Update(task); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 发送抽帧任务开始事件
	if s.sseService != nil {
		metadata := map[string]interface{}{
			"task_id":   task.ID,
			"task_name": task.Name,
			"user_id":   task.UserID,
			"status":    task.Status,
			"source":    "extract_task_service",
			"source_id": fmt.Sprintf("extract_task_%d", task.ID),
			"title":     "抽帧任务已开始",
			"message":   fmt.Sprintf("抽帧任务 %d 已开始执行", task.ID),
		}
		s.sseService.BroadcastExtractTaskStarted(task.ID, metadata)
	}

	// 异步执行抽帧
	go func() {
		if err := s.executeExtractTask(context.Background(), task); err != nil {
			task.MarkAsFailed(err.Error())
			s.taskRepo.Update(task)

			// 发送抽帧任务失败事件
			if s.sseService != nil {
				metadata := map[string]interface{}{
					"task_id":   task.ID,
					"task_name": task.Name,
					"user_id":   task.UserID,
					"error":     err.Error(),
					"source":    "extract_task_service",
					"source_id": fmt.Sprintf("extract_task_%d", task.ID),
					"title":     "抽帧任务失败",
					"message":   fmt.Sprintf("抽帧任务 %d 执行失败: %v", task.ID, err),
				}
				s.sseService.BroadcastExtractTaskFailed(task.ID, err.Error(), metadata)
			}
		}
	}()

	return nil
}

// StopExtractTask 停止抽帧任务
func (s *extractTaskService) StopExtractTask(ctx context.Context, id uint) error {
	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("获取抽帧任务失败: %w", err)
	}

	if !task.CanStop() {
		return fmt.Errorf("任务状态不允许停止: %s", task.Status)
	}

	// 标记任务为已停止
	task.MarkAsStopped()
	return s.taskRepo.Update(task)
}

// DeleteExtractTask 删除抽帧任务
func (s *extractTaskService) DeleteExtractTask(ctx context.Context, id uint) error {
	task, err := s.taskRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("获取抽帧任务失败: %w", err)
	}

	// 检查任务是否可以删除
	if task.IsActive() {
		return fmt.Errorf("无法删除正在执行的任务")
	}

	// 删除相关的抽帧记录
	if task.TaskID != "" {
		if frames, err := s.frameRepo.GetByTaskID(task.TaskID); err == nil {
			for _, frame := range frames {
				// 删除文件
				if frame.FramePath != "" {
					os.Remove(frame.FramePath)
				}
				// 删除记录
				s.frameRepo.Delete(frame.ID)
			}
		}
	}

	// 删除输出目录
	if task.OutputDir != "" {
		os.RemoveAll(task.OutputDir)
	}

	// 删除任务
	return s.taskRepo.Delete(id)
}

// GetTaskFrames 获取任务的抽帧结果
func (s *extractTaskService) GetTaskFrames(ctx context.Context, taskID string, page, pageSize int) ([]*models.ExtractFrame, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	taskIDPtr := &taskID
	return s.frameRepo.List(page, pageSize, taskIDPtr)
}

// GetFrameImage 获取抽帧图片信息
func (s *extractTaskService) GetFrameImage(ctx context.Context, frameID uint) (*FrameImageInfo, error) {
	log.Printf("[DEBUG] GetFrameImage called for frame ID: %d", frameID)

	// 获取抽帧记录
	frame, err := s.frameRepo.GetByID(frameID)
	if err != nil {
		log.Printf("[ERROR] Failed to get frame %d: %v", frameID, err)
		return nil, fmt.Errorf("获取抽帧记录失败: %w", err)
	}
	log.Printf("[DEBUG] Found frame - ID: %d, FramePath: %s, Status: %s", frame.ID, frame.FramePath, frame.Status)

	// 检查抽帧状态
	if frame.Status != models.ExtractFrameStatusExtracted {
		log.Printf("[ERROR] Frame %d is not extracted, current status: %s", frameID, frame.Status)
		return nil, fmt.Errorf("抽帧文件未就绪")
	}

	// 检查文件路径
	if frame.FramePath == "" {
		log.Printf("[ERROR] Frame %d has no file path", frameID)
		return nil, fmt.Errorf("抽帧文件路径为空")
	}

	// 检查文件是否存在
	log.Printf("[DEBUG] Checking if frame file exists: %s", frame.FramePath)
	fileInfo, err := os.Stat(frame.FramePath)
	if os.IsNotExist(err) {
		log.Printf("[ERROR] Frame file does not exist for frame %d: %s", frameID, frame.FramePath)
		return nil, fmt.Errorf("抽帧文件不存在")
	}
	if err != nil {
		log.Printf("[ERROR] Failed to stat frame file %s: %v", frame.FramePath, err)
		return nil, fmt.Errorf("检查抽帧文件失败: %w", err)
	}

	actualFileSize := fileInfo.Size()
	log.Printf("[DEBUG] Frame file exists - Path: %s, Size: %d bytes, Database FileSize: %d bytes",
		frame.FramePath, actualFileSize, frame.FileSize)

	// 确定Content-Type
	contentType := "image/jpeg" // 默认为JPEG
	switch frame.Format {
	case "jpg", "jpeg":
		contentType = "image/jpeg"
	case "png":
		contentType = "image/png"
	case "bmp":
		contentType = "image/bmp"
	case "gif":
		contentType = "image/gif"
	case "webp":
		contentType = "image/webp"
	}

	frameImageInfo := &FrameImageInfo{
		FilePath:    frame.FramePath,
		FileName:    filepath.Base(frame.FramePath),
		ContentType: contentType,
		FileSize:    frame.FileSize,
		Width:       frame.Width,
		Height:      frame.Height,
		Format:      frame.Format,
		Timestamp:   frame.GetFormattedTimestamp(),
	}

	log.Printf("[DEBUG] GetFrameImage prepared - FrameID: %d, FileName: %s, FileSize: %d, ContentType: %s, Resolution: %dx%d",
		frameID, frameImageInfo.FileName, frameImageInfo.FileSize, frameImageInfo.ContentType, frameImageInfo.Width, frameImageInfo.Height)

	return frameImageInfo, nil
}

// DownloadTaskImages 下载任务的所有抽帧图片（压缩为zip包）
func (s *extractTaskService) DownloadTaskImages(ctx context.Context, taskID uint) (*TaskImagesDownloadInfo, error) {
	log.Printf("[DEBUG] DownloadTaskImages called for task ID: %d", taskID)

	// 获取任务信息
	task, err := s.taskRepo.GetByID(taskID)
	if err != nil {
		log.Printf("[ERROR] Failed to get extract task %d: %v", taskID, err)
		return nil, fmt.Errorf("获取抽帧任务失败: %w", err)
	}
	log.Printf("[DEBUG] Found extract task - ID: %d, TaskID: %s, Status: %s", task.ID, task.TaskID, task.Status)

	// 检查任务状态
	if task.Status != models.ExtractTaskStatusCompleted {
		log.Printf("[ERROR] Extract task %d is not completed, current status: %s", taskID, task.Status)
		return nil, fmt.Errorf("抽帧任务未完成，无法下载")
	}

	// 获取任务的所有抽帧记录
	log.Printf("[DEBUG] Getting all frames for task %s", task.TaskID)
	frames, err := s.frameRepo.GetByTaskID(task.TaskID)
	if err != nil {
		log.Printf("[ERROR] Failed to get frames for task %s: %v", task.TaskID, err)
		return nil, fmt.Errorf("获取抽帧记录失败: %w", err)
	}

	if len(frames) == 0 {
		log.Printf("[ERROR] No frames found for task %s", task.TaskID)
		return nil, fmt.Errorf("任务没有抽帧结果")
	}

	log.Printf("[DEBUG] Found %d frames for task %s", len(frames), task.TaskID)

	// 过滤出有效的抽帧记录
	var validFrames []*models.ExtractFrame
	for _, frame := range frames {
		if frame.Status == models.ExtractFrameStatusExtracted && frame.FramePath != "" {
			// 检查文件是否存在
			if _, err := os.Stat(frame.FramePath); err == nil {
				validFrames = append(validFrames, frame)
			} else {
				log.Printf("[WARNING] Frame file not found for frame %d: %s", frame.ID, frame.FramePath)
			}
		}
	}

	if len(validFrames) == 0 {
		log.Printf("[ERROR] No valid frames found for task %s", task.TaskID)
		return nil, fmt.Errorf("没有可用的抽帧文件")
	}

	log.Printf("[DEBUG] Found %d valid frames for task %s", len(validFrames), task.TaskID)

	// 创建临时zip文件
	zipFileName := fmt.Sprintf("extract_task_%s_frames_%s.zip", task.TaskID, time.Now().Format("20060102_150405"))
	zipDir := filepath.Join(s.basePath, "temp", "downloads")
	if err := os.MkdirAll(zipDir, 0755); err != nil {
		log.Printf("[ERROR] Failed to create zip directory %s: %v", zipDir, err)
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	zipFilePath := filepath.Join(zipDir, zipFileName)
	log.Printf("[DEBUG] Creating zip file: %s", zipFilePath)

	// 创建zip文件
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		log.Printf("[ERROR] Failed to create zip file %s: %v", zipFilePath, err)
		return nil, fmt.Errorf("创建zip文件失败: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 添加抽帧图片到zip文件
	var totalSize int64
	successCount := 0

	for i, frame := range validFrames {
		log.Printf("[DEBUG] Processing frame %d/%d - FrameID: %d, FramePath: %s",
			i+1, len(validFrames), frame.ID, frame.FramePath)

		// 打开源文件
		srcFile, err := os.Open(frame.FramePath)
		if err != nil {
			log.Printf("[WARNING] Failed to open frame file %s: %v", frame.FramePath, err)
			continue
		}

		// 获取文件信息
		_, err = srcFile.Stat()
		if err != nil {
			log.Printf("[WARNING] Failed to get file info for %s: %v", frame.FramePath, err)
			srcFile.Close()
			continue
		}

		// 在zip中创建文件条目
		// 使用更友好的文件名：frame_序号_时间戳.格式
		zipEntryName := fmt.Sprintf("frame_%05d_%s.%s",
			frame.FrameIndex,
			strings.Replace(frame.GetFormattedTimestamp(), ":", "-", -1),
			frame.Format)

		zipEntry, err := zipWriter.Create(zipEntryName)
		if err != nil {
			log.Printf("[WARNING] Failed to create zip entry %s: %v", zipEntryName, err)
			srcFile.Close()
			continue
		}

		// 复制文件内容到zip
		bytesWritten, err := io.Copy(zipEntry, srcFile)
		srcFile.Close()

		if err != nil {
			log.Printf("[WARNING] Failed to copy file content for %s: %v", frame.FramePath, err)
			continue
		}

		totalSize += bytesWritten
		successCount++
		log.Printf("[DEBUG] Added frame to zip - FrameID: %d, ZipEntry: %s, Size: %d bytes",
			frame.ID, zipEntryName, bytesWritten)
	}

	// 关闭zip writer以确保数据写入完成
	if err := zipWriter.Close(); err != nil {
		log.Printf("[ERROR] Failed to close zip writer: %v", err)
		return nil, fmt.Errorf("完成zip文件创建失败: %w", err)
	}

	// 获取最终zip文件大小
	zipFileInfo, err := zipFile.Stat()
	if err != nil {
		log.Printf("[ERROR] Failed to get zip file info: %v", err)
		return nil, fmt.Errorf("获取zip文件信息失败: %w", err)
	}

	if successCount == 0 {
		// 删除空的zip文件
		os.Remove(zipFilePath)
		log.Printf("[ERROR] No frames were successfully added to zip for task %s", task.TaskID)
		return nil, fmt.Errorf("没有成功添加任何抽帧文件到压缩包")
	}

	taskImagesDownloadInfo := &TaskImagesDownloadInfo{
		ZipFilePath: zipFilePath,
		ZipFileName: zipFileName,
		ZipFileSize: zipFileInfo.Size(),
		ImageCount:  successCount,
		TaskID:      task.TaskID,
		ContentType: "application/zip",
	}

	log.Printf("[DEBUG] DownloadTaskImages completed successfully - TaskID: %s, ZipFile: %s, ZipSize: %d bytes, ImageCount: %d",
		task.TaskID, zipFileName, zipFileInfo.Size(), successCount)

	return taskImagesDownloadInfo, nil
}

// validateDataSource 验证数据源
func (s *extractTaskService) validateDataSource(ctx context.Context, task *models.ExtractTask) error {
	if task.VideoSourceID != nil {
		// 验证视频源
		vs, err := s.videoSourceRepo.GetByID(*task.VideoSourceID)
		if err != nil {
			return fmt.Errorf("视频源不存在: %w", err)
		}

		// 检查视频源状态
		if vs.Status != models.VideoSourceStatusActive {
			return fmt.Errorf("视频源未激活")
		}
	} else if task.VideoFileID != nil {
		// 验证视频文件
		vf, err := s.videoFileRepo.GetByID(*task.VideoFileID)
		if err != nil {
			return fmt.Errorf("视频文件不存在: %w", err)
		}

		// 检查视频文件状态
		if vf.Status != models.VideoFileStatusReady {
			return fmt.Errorf("视频文件未就绪")
		}
	}

	return nil
}

// executeExtractTask 执行抽帧任务
func (s *extractTaskService) executeExtractTask(ctx context.Context, task *models.ExtractTask) error {
	log.Printf("[ExtractTaskService] executeExtractTask started - TaskID: %s, Name: %s, UserID: %d",
		task.TaskID, task.Name, task.UserID)

	// 获取输入源
	log.Printf("[ExtractTaskService] Getting input path - TaskID: %s, VideoSourceID: %v, VideoFileID: %v",
		task.TaskID, task.VideoSourceID, task.VideoFileID)
	inputPath, err := s.getInputPath(ctx, task)
	if err != nil {
		log.Printf("[ExtractTaskService] Failed to get input path - TaskID: %s, Error: %v", task.TaskID, err)
		return fmt.Errorf("获取输入路径失败: %w", err)
	}
	log.Printf("[ExtractTaskService] Input path resolved - TaskID: %s, InputPath: %s", task.TaskID, inputPath)

	// 创建输出目录
	outputDir := filepath.Join(s.basePath, "tasks", task.TaskID)
	log.Printf("[ExtractTaskService] Creating output directory - TaskID: %s, OutputDir: %s", task.TaskID, outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("[ExtractTaskService] Failed to create output directory - TaskID: %s, OutputDir: %s, Error: %v",
			task.TaskID, outputDir, err)
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 配置FFmpeg参数
	log.Printf("[ExtractTaskService] Configuring FFmpeg parameters - TaskID: %s, Quality: %d, Format: %s",
		task.TaskID, task.Quality, task.Format)
	extractReq := &ffmpeg.ExtractFramesRequest{
		InputPath:   inputPath,
		OutputDir:   outputDir,
		Quality:     task.Quality,
		Format:      task.Format,
		NamePattern: "frame_%05d",
	}

	// 设置抽帧参数
	if task.FrameCount != nil {
		extractReq.FrameCount = *task.FrameCount
		log.Printf("[ExtractTaskService] Frame count set - TaskID: %s, FrameCount: %d", task.TaskID, *task.FrameCount)
	}
	if task.Duration != nil {
		extractReq.Duration = *task.Duration
		log.Printf("[ExtractTaskService] Duration set - TaskID: %s, Duration: %d", task.TaskID, *task.Duration)
	}
	if task.StartTime != nil {
		extractReq.StartTime = float64(*task.StartTime)
		log.Printf("[ExtractTaskService] Start time set - TaskID: %s, StartTime: %d", task.TaskID, *task.StartTime)
	}

	log.Printf("[ExtractTaskService] FFmpeg parameters configured - TaskID: %s, InputPath: %s, OutputDir: %s, Quality: %d, Format: %s",
		task.TaskID, extractReq.InputPath, extractReq.OutputDir, extractReq.Quality, extractReq.Format)

	// 执行抽帧
	log.Printf("[ExtractTaskService] Starting FFmpeg frame extraction - TaskID: %s", task.TaskID)
	result, err := s.ffmpegModule.ExtractFrames(ctx, extractReq)
	if err != nil {
		log.Printf("[ExtractTaskService] FFmpeg frame extraction failed - TaskID: %s, Error: %v", task.TaskID, err)
		return fmt.Errorf("抽帧执行失败: %w", err)
	}

	log.Printf("[ExtractTaskService] FFmpeg frame extraction completed successfully - TaskID: %s, FrameCount: %d, Duration: %f",
		task.TaskID, result.FrameCount, result.Duration)

	// 保存抽帧结果
	log.Printf("[ExtractTaskService] Starting to save frame results - TaskID: %s, FrameCount: %d",
		task.TaskID, len(result.FramePaths))
	var totalSize int64
	successCount := 0
	for i, framePath := range result.FramePaths {
		log.Printf("[ExtractTaskService] Processing frame %d/%d - TaskID: %s, FramePath: %s",
			i+1, len(result.FramePaths), task.TaskID, framePath)

		// 获取文件信息
		fileInfo, err := os.Stat(framePath)
		fileSize := int64(0)
		if err != nil {
			log.Printf("[ExtractTaskService] Failed to get frame file info - TaskID: %s, FramePath: %s, Error: %v",
				task.TaskID, framePath, err)
		} else {
			fileSize = fileInfo.Size()
			totalSize += fileSize
			log.Printf("[ExtractTaskService] Frame file info - TaskID: %s, FramePath: %s, Size: %d bytes",
				task.TaskID, framePath, fileSize)
		}

		// 创建抽帧记录
		frame := &models.ExtractFrame{
			TaskID:     task.TaskID,
			Name:       fmt.Sprintf("frame_%05d", i+1),
			FramePath:  framePath,
			FrameIndex: int64(i + 1),
			Timestamp:  float64(i) * (result.Duration / float64(result.FrameCount)),
			FileSize:   fileSize,
			Width:      1920, // 后续可从FFmpeg获取实际尺寸
			Height:     1080,
			Format:     task.Format,
			UserID:     task.UserID,
			Status:     models.ExtractFrameStatusExtracted,
		}

		// 设置关联ID
		if task.VideoSourceID != nil {
			frame.VideoSourceID = *task.VideoSourceID
			log.Printf("[ExtractTaskService] Frame associated with video source - TaskID: %s, FrameIndex: %d, VideoSourceID: %d",
				task.TaskID, frame.FrameIndex, *task.VideoSourceID)
		}

		// 保存抽帧记录
		if err := s.frameRepo.Create(frame); err != nil {
			// 记录错误但继续处理其他帧
			log.Printf("[ExtractTaskService] Failed to save frame record - TaskID: %s, FrameIndex: %d, Error: %v",
				task.TaskID, frame.FrameIndex, err)
		} else {
			successCount++
			log.Printf("[ExtractTaskService] Frame record saved successfully - TaskID: %s, FrameIndex: %d, FrameID: %d",
				task.TaskID, frame.FrameIndex, frame.ID)
		}
	}

	log.Printf("[ExtractTaskService] Frame processing completed - TaskID: %s, TotalFrames: %d, SavedFrames: %d, TotalSize: %d bytes",
		task.TaskID, len(result.FramePaths), successCount, totalSize)

	// 标记任务完成
	log.Printf("[ExtractTaskService] Marking task as completed - TaskID: %s, OutputDir: %s, FrameCount: %d, TotalSize: %d",
		task.TaskID, outputDir, result.FrameCount, totalSize)
	task.MarkAsCompleted(outputDir, result.FrameCount, totalSize)

	if err := s.taskRepo.Update(task); err != nil {
		log.Printf("[ExtractTaskService] Failed to update task completion status - TaskID: %s, Error: %v", task.TaskID, err)
		return err
	}

	// 发送抽帧任务完成事件
	if s.sseService != nil {
		metadata := map[string]interface{}{
			"task_id":     task.ID,
			"task_name":   task.Name,
			"user_id":     task.UserID,
			"frame_count": result.FrameCount,
			"total_size":  totalSize,
			"output_dir":  outputDir,
			"source":      "extract_task_service",
			"source_id":   fmt.Sprintf("extract_task_%d", task.ID),
			"title":       "抽帧任务已完成",
			"message":     fmt.Sprintf("抽帧任务 %d 已成功完成，共抽取 %d 帧", task.ID, result.FrameCount),
		}
		s.sseService.BroadcastExtractTaskCompleted(task.ID, fmt.Sprintf("成功抽取 %d 帧", result.FrameCount), metadata)
	}

	log.Printf("[ExtractTaskService] executeExtractTask completed successfully - TaskID: %s, Status: %s, OutputDir: %s",
		task.TaskID, task.Status, task.OutputDir)
	return nil
}

// getInputPath 获取输入路径
func (s *extractTaskService) getInputPath(ctx context.Context, task *models.ExtractTask) (string, error) {
	if task.VideoSourceID != nil {
		// 视频源
		vs, err := s.videoSourceRepo.GetByID(*task.VideoSourceID)
		if err != nil {
			return "", err
		}

		if vs.Type == models.VideoSourceTypeStream {
			// 实时流
			return vs.PlayURL, nil
		} else {
			// 文件类型视频源
			return "", fmt.Errorf("文件类型视频源请使用video_file_id参数")
		}
	} else if task.VideoFileID != nil {
		// 视频文件
		vf, err := s.videoFileRepo.GetByID(*task.VideoFileID)
		if err != nil {
			return "", err
		}

		// 优先使用转换后的文件
		filePath := vf.OriginalPath
		if vf.ConvertedPath != "" {
			filePath = vf.ConvertedPath
		}

		// 处理路径：确保返回完整可访问的路径
		return s.resolveVideoFilePath(filePath)
	}

	return "", fmt.Errorf("未指定有效的数据源")
}

// resolveVideoFilePath 解析视频文件路径，确保返回完整可访问的路径
func (s *extractTaskService) resolveVideoFilePath(filePath string) (string, error) {
	// 如果是绝对路径，直接检查并返回
	if filepath.IsAbs(filePath) {
		if _, err := os.Stat(filePath); err != nil {
			return "", fmt.Errorf("视频文件不存在: %s", filePath)
		}
		return filePath, nil
	}

	// 相对路径处理：尝试多种可能的基础路径
	possiblePaths := []string{
		filePath, // 首先尝试原路径（相对于当前工作目录）
	}

	// 如果有配置的视频基础路径，尝试基于它构建路径
	if s.videoBasePath != "" {
		// 尝试直接拼接
		possiblePaths = append(possiblePaths, filepath.Join(s.videoBasePath, filePath))

		// 如果filePath已经包含了某些目录前缀，尝试提取文件名重新拼接
		// 例如：filePath = "data/video/converted/xxx.mp4"，提取 "xxx.mp4" 或 "converted/xxx.mp4"
		parts := strings.Split(filePath, string(filepath.Separator))
		for i := range parts {
			subPath := filepath.Join(parts[i:]...)
			possiblePaths = append(possiblePaths, filepath.Join(s.videoBasePath, subPath))
		}
	}

	// 尝试所有可能的路径
	for _, path := range possiblePaths {
		log.Printf("[ExtractTaskService] Trying path: %s", path)
		if _, err := os.Stat(path); err == nil {
			log.Printf("[ExtractTaskService] Found valid path: %s", path)
			return path, nil
		}
	}

	return "", fmt.Errorf("无法找到视频文件，尝试的路径: %v", possiblePaths)
}
