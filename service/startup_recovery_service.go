/*
 * @module service/startup_recovery_service
 * @description 程序启动时的恢复服务，处理异常退出导致的不一致状态
 * @architecture 服务层
 * @documentReference REQ-001: 视频文件管理功能
 * @stateFlow 程序启动 -> 检测异常状态 -> 恢复一致性 -> 完成初始化
 * @rules 重置转换中的任务状态、清理临时文件、恢复服务状态
 * @dependencies box-manage-service/repository, box-manage-service/models
 * @refs REQ-001.md
 */

package service

import (
	"box-manage-service/models"
	"box-manage-service/repository"
	"context"
	"fmt"
	"log"
	"time"
)

// startupRecoveryService 启动恢复服务实现
type startupRecoveryService struct {
	videoFileRepo      repository.VideoFileRepository
	conversionRepo     repository.ConversionTaskRepository
	extractTaskRepo    repository.ExtractTaskRepository
	recordTaskRepo     repository.RecordTaskRepository
	videoSourceService VideoSourceService
}

// NewStartupRecoveryService 创建启动恢复服务
func NewStartupRecoveryService(
	videoFileRepo repository.VideoFileRepository,
	conversionRepo repository.ConversionTaskRepository,
	extractTaskRepo repository.ExtractTaskRepository,
	recordTaskRepo repository.RecordTaskRepository,
	videoSourceService VideoSourceService,
) StartupRecoveryService {
	return &startupRecoveryService{
		videoFileRepo:      videoFileRepo,
		conversionRepo:     conversionRepo,
		extractTaskRepo:    extractTaskRepo,
		recordTaskRepo:     recordTaskRepo,
		videoSourceService: videoSourceService,
	}
}

// RecoverOnStartup 程序启动时执行恢复操作
func (s *startupRecoveryService) RecoverOnStartup(ctx context.Context) error {
	log.Printf("[StartupRecoveryService] Starting recovery process on application startup")

	// 记录开始时间
	startTime := time.Now()

	// 1. 恢复视频文件转换状态
	if err := s.recoverVideoFileConversions(ctx); err != nil {
		log.Printf("[StartupRecoveryService] Failed to recover video file conversions - Error: %v", err)
		return fmt.Errorf("恢复视频文件转换状态失败: %w", err)
	}

	// 2. 恢复模型转换任务状态
	if err := s.recoverConversionTasks(ctx); err != nil {
		log.Printf("[StartupRecoveryService] Failed to recover conversion tasks - Error: %v", err)
		return fmt.Errorf("恢复模型转换任务状态失败: %w", err)
	}

	// 3. 恢复抽帧任务状态
	if err := s.recoverExtractTasks(ctx); err != nil {
		log.Printf("[StartupRecoveryService] Failed to recover extract tasks - Error: %v", err)
		return fmt.Errorf("恢复抽帧任务状态失败: %w", err)
	}

	// 4. 恢复录制任务状态
	if err := s.recoverRecordTasks(ctx); err != nil {
		log.Printf("[StartupRecoveryService] Failed to recover record tasks - Error: %v", err)
		return fmt.Errorf("恢复录制任务状态失败: %w", err)
	}

	// 5. 同步视频源到ZLMediaKit
	if err := s.syncVideoSourcesToZLMediaKit(ctx); err != nil {
		log.Printf("[StartupRecoveryService] Failed to sync video sources - Error: %v", err)
		// 视频源同步失败不影响整体启动过程
	}

	elapsed := time.Since(startTime)
	log.Printf("[StartupRecoveryService] Recovery process completed successfully - Duration: %s", elapsed.String())

	return nil
}

// recoverVideoFileConversions 恢复视频文件转换状态
func (s *startupRecoveryService) recoverVideoFileConversions(ctx context.Context) error {
	log.Printf("[StartupRecoveryService] Starting video file conversion recovery")

	// 查找所有转换中状态的视频文件
	convertingFiles, err := s.videoFileRepo.FindByStatus(models.VideoFileStatusConverting)
	if err != nil {
		log.Printf("[StartupRecoveryService] Failed to find converting video files - Error: %v", err)
		return fmt.Errorf("查找转换中的视频文件失败: %w", err)
	}

	if len(convertingFiles) == 0 {
		log.Printf("[StartupRecoveryService] No converting video files found to recover")
		return nil
	}

	log.Printf("[StartupRecoveryService] Found %d converting video files to recover", len(convertingFiles))

	// 将所有转换中的文件状态设置为转换失败
	recoveredCount := 0
	for _, vf := range convertingFiles {
		log.Printf("[StartupRecoveryService] Recovering video file - ID: %d, Name: %s, Status: %s",
			vf.ID, vf.Name, vf.Status)

		// 设置为转换失败状态
		vf.MarkAsError("程序重启导致转换中断")

		// 重置进度
		vf.ProcessProgress = 0

		// 更新到数据库
		if err := s.videoFileRepo.Update(vf); err != nil {
			log.Printf("[StartupRecoveryService] Failed to update video file status - ID: %d, Error: %v", vf.ID, err)
			continue
		}

		recoveredCount++
		log.Printf("[StartupRecoveryService] Video file recovered successfully - ID: %d, NewStatus: %s", vf.ID, vf.Status)
	}

	log.Printf("[StartupRecoveryService] Video file conversion recovery completed - Total: %d, Recovered: %d",
		len(convertingFiles), recoveredCount)

	return nil
}

// recoverConversionTasks 恢复模型转换任务状态
func (s *startupRecoveryService) recoverConversionTasks(ctx context.Context) error {
	log.Printf("[StartupRecoveryService] Starting conversion task recovery")

	// 查找所有运行中状态的转换任务
	runningTasks, err := s.conversionRepo.GetRunningTasks(ctx)
	if err != nil {
		log.Printf("[StartupRecoveryService] Failed to find running conversion tasks - Error: %v", err)
		return fmt.Errorf("查找运行中的转换任务失败: %w", err)
	}

	if len(runningTasks) == 0 {
		log.Printf("[StartupRecoveryService] No running conversion tasks found to recover")
		return nil
	}

	log.Printf("[StartupRecoveryService] Found %d running conversion tasks to recover", len(runningTasks))

	// 将所有运行中的任务状态设置为失败
	recoveredCount := 0
	for _, task := range runningTasks {
		log.Printf("[StartupRecoveryService] Recovering conversion task - TaskID: %s, Status: %s",
			task.TaskID, task.Status)

		// 设置为失败状态
		task.Fail("程序重启导致转换中断")

		// 更新到数据库
		if err := s.conversionRepo.Update(ctx, task); err != nil {
			log.Printf("[StartupRecoveryService] Failed to update conversion task status - TaskID: %s, Error: %v",
				task.TaskID, err)
			continue
		}

		recoveredCount++
		log.Printf("[StartupRecoveryService] Conversion task recovered successfully - TaskID: %s, NewStatus: %s",
			task.TaskID, task.Status)
	}

	log.Printf("[StartupRecoveryService] Conversion task recovery completed - Total: %d, Recovered: %d",
		len(runningTasks), recoveredCount)

	return nil
}

// recoverExtractTasks 恢复抽帧任务状态
func (s *startupRecoveryService) recoverExtractTasks(ctx context.Context) error {
	log.Printf("[StartupRecoveryService] Starting extract task recovery")

	// 查找所有执行中状态的抽帧任务
	extractingTasks, err := s.extractTaskRepo.FindByStatus(models.ExtractTaskStatusExtracting)
	if err != nil {
		log.Printf("[StartupRecoveryService] Failed to find extracting tasks - Error: %v", err)
		return fmt.Errorf("查找执行中的抽帧任务失败: %w", err)
	}

	if len(extractingTasks) == 0 {
		log.Printf("[StartupRecoveryService] No extracting tasks found to recover")
		return nil
	}

	log.Printf("[StartupRecoveryService] Found %d extracting tasks to recover", len(extractingTasks))

	// 将所有执行中的任务状态设置为失败
	recoveredCount := 0
	for _, task := range extractingTasks {
		log.Printf("[StartupRecoveryService] Recovering extract task - ID: %d, TaskID: %s, Status: %s",
			task.ID, task.TaskID, task.Status)

		// 设置为失败状态
		task.MarkAsFailed("程序重启导致抽帧中断")

		// 更新到数据库
		if err := s.extractTaskRepo.Update(task); err != nil {
			log.Printf("[StartupRecoveryService] Failed to update extract task status - ID: %d, Error: %v",
				task.ID, err)
			continue
		}

		recoveredCount++
		log.Printf("[StartupRecoveryService] Extract task recovered successfully - ID: %d, NewStatus: %s",
			task.ID, task.Status)
	}

	log.Printf("[StartupRecoveryService] Extract task recovery completed - Total: %d, Recovered: %d",
		len(extractingTasks), recoveredCount)

	return nil
}

// recoverRecordTasks 恢复录制任务状态
func (s *startupRecoveryService) recoverRecordTasks(ctx context.Context) error {
	log.Printf("[StartupRecoveryService] Starting record task recovery")

	// 查找所有录制中状态的录制任务
	recordingTasks, err := s.recordTaskRepo.FindByStatus(models.RecordTaskStatusRecording)
	if err != nil {
		log.Printf("[StartupRecoveryService] Failed to find recording tasks - Error: %v", err)
		return fmt.Errorf("查找录制中的任务失败: %w", err)
	}

	if len(recordingTasks) == 0 {
		log.Printf("[StartupRecoveryService] No recording tasks found to recover")
		return nil
	}

	log.Printf("[StartupRecoveryService] Found %d recording tasks to recover", len(recordingTasks))

	// 将所有录制中的任务状态设置为停止
	recoveredCount := 0
	for _, task := range recordingTasks {
		log.Printf("[StartupRecoveryService] Recovering record task - ID: %d, Name: %s, Status: %s",
			task.ID, task.Name, task.Status)

		// 设置为停止状态
		task.MarkAsStopped()

		// 更新到数据库
		if err := s.recordTaskRepo.Update(task); err != nil {
			log.Printf("[StartupRecoveryService] Failed to update record task status - ID: %d, Error: %v",
				task.ID, err)
			continue
		}

		recoveredCount++
		log.Printf("[StartupRecoveryService] Record task recovered successfully - ID: %d, NewStatus: %s",
			task.ID, task.Status)
	}

	log.Printf("[StartupRecoveryService] Record task recovery completed - Total: %d, Recovered: %d",
		len(recordingTasks), recoveredCount)

	return nil
}

// syncVideoSourcesToZLMediaKit 同步视频源到ZLMediaKit
func (s *startupRecoveryService) syncVideoSourcesToZLMediaKit(ctx context.Context) error {
	log.Printf("[StartupRecoveryService] Starting video sources sync to ZLMediaKit")

	if s.videoSourceService == nil {
		log.Printf("[StartupRecoveryService] VideoSourceService is nil, skipping sync")
		return nil
	}

	// 调用视频源服务的同步方法
	if err := s.videoSourceService.SyncToZLMediaKit(ctx); err != nil {
		log.Printf("[StartupRecoveryService] Failed to sync video sources to ZLMediaKit - Error: %v", err)
		return fmt.Errorf("同步视频源到ZLMediaKit失败: %w", err)
	}

	log.Printf("[StartupRecoveryService] Video sources sync to ZLMediaKit completed successfully")
	return nil
}
