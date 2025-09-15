/*
 * @module service/upgrade_service
 * @description 升级管理服务，实现盒子软件升级、回滚、进度跟踪等功能
 * @architecture 服务层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow 升级任务创建 -> 执行升级 -> 进度跟踪 -> 完成/失败 -> 回滚（可选）
 * @rules 支持单个和批量升级、进度监控、版本回滚、错误处理等功能
 * @dependencies gorm.io/gorm, net/http
 * @refs DESIGN-001.md
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

// UpgradeService 升级管理服务
type UpgradeService struct {
	repoManager    repository.RepositoryManager
	proxyService   *BoxProxyService
	monitorService *BoxMonitoringService
}

// NewUpgradeService 创建升级管理服务
func NewUpgradeService(repoManager repository.RepositoryManager, proxyService *BoxProxyService, monitorService *BoxMonitoringService) *UpgradeService {
	return &UpgradeService{
		repoManager:    repoManager,
		proxyService:   proxyService,
		monitorService: monitorService,
	}
}

// CreateUpgradeTask 创建升级任务
func (s *UpgradeService) CreateUpgradeTask(boxID uint, upgradePackageID uint, force bool, createdBy uint) (*models.UpgradeTask, error) {
	ctx := context.Background()

	// 获取升级包信息
	upgradePackage, err := s.repoManager.UpgradePackage().GetByID(ctx, upgradePackageID)
	if err != nil {
		return nil, fmt.Errorf("查找升级包失败: %w", err)
	}
	if upgradePackage == nil {
		return nil, fmt.Errorf("升级包 %d 不存在", upgradePackageID)
	}

	// 获取盒子信息
	box, err := s.repoManager.Box().GetByID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("查找盒子失败: %w", err)
	}

	// 获取当前版本
	versionResp, err := s.proxyService.GetSystemVersion(boxID)
	if err != nil {
		return nil, fmt.Errorf("获取当前版本失败: %w", err)
	}

	var currentVersion string
	if versionResp != nil {
		currentVersion = versionResp.Version
	}

	// 检查是否有正在进行的升级任务
	existingTasks, err := s.repoManager.Upgrade().FindByBoxID(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("检查升级任务失败: %w", err)
	}

	for _, task := range existingTasks {
		if task.Status == models.UpgradeStatusPending || task.Status == models.UpgradeStatusRunning {
			return nil, fmt.Errorf("盒子 %s 已有正在进行的升级任务", box.Name)
		}
	}

	// 创建升级任务
	task := &models.UpgradeTask{
		BoxID:            boxID,
		Name:             fmt.Sprintf("升级盒子 %s 到版本 %s", box.Name, upgradePackage.Version),
		VersionFrom:      currentVersion,
		VersionTo:        upgradePackage.Version,
		Status:           models.UpgradeStatusPending,
		UpgradePackageID: upgradePackageID,
		Force:            force,
		CreatedBy:        createdBy,
	}

	if err := s.repoManager.Upgrade().Create(ctx, task); err != nil {
		return nil, fmt.Errorf("创建升级任务失败: %w", err)
	}

	log.Printf("已创建升级任务: 盒子 %s 从版本 %s 升级到 %s (升级包ID: %d)", box.Name, currentVersion, upgradePackage.Version, upgradePackageID)
	return task, nil
}

// CreateBatchUpgradeTask 创建批量升级任务
func (s *UpgradeService) CreateBatchUpgradeTask(boxIDs []uint, upgradePackageID uint, force bool, createdBy uint) (*models.BatchUpgradeTask, error) {
	if len(boxIDs) == 0 {
		return nil, fmt.Errorf("盒子列表不能为空")
	}

	ctx := context.Background()

	// 获取升级包信息
	upgradePackage, err := s.repoManager.UpgradePackage().GetByID(ctx, upgradePackageID)
	if err != nil {
		return nil, fmt.Errorf("查找升级包失败: %w", err)
	}
	if upgradePackage == nil {
		return nil, fmt.Errorf("升级包 %d 不存在", upgradePackageID)
	}

	// 验证所有盒子存在
	for _, boxID := range boxIDs {
		_, err := s.repoManager.Box().GetByID(ctx, boxID)
		if err != nil {
			return nil, fmt.Errorf("盒子 %d 不存在: %w", boxID, err)
		}
	}

	// 创建批量任务
	batchTask := &models.BatchUpgradeTask{
		Name:             fmt.Sprintf("批量升级 %d 个盒子到版本 %s", len(boxIDs), upgradePackage.Version),
		BoxIDs:           models.BoxIDList(boxIDs),
		VersionTo:        upgradePackage.Version,
		Status:           models.UpgradeStatusPending,
		TotalBoxes:       len(boxIDs),
		UpgradePackageID: upgradePackageID,
		Force:            force,
		CreatedBy:        createdBy,
	}

	// 这里需要在repository中添加BatchUpgrade的创建方法
	// 暂时先注释掉，等repository接口完善后再处理
	/*
		if err := s.repoManager.BatchUpgrade().Create(ctx, batchTask); err != nil {
			return nil, fmt.Errorf("创建批量升级任务失败: %w", err)
		}
	*/

	// 为每个盒子创建单独的升级任务
	for _, boxID := range boxIDs {
		task, err := s.CreateUpgradeTask(boxID, upgradePackageID, force, createdBy)
		if err != nil {
			log.Printf("为盒子 %d 创建升级任务失败: %v", boxID, err)
			continue
		}

		// 关联到批量任务
		task.BatchUpgradeTaskID = &batchTask.ID
		s.repoManager.Upgrade().Update(ctx, task)
	}

	log.Printf("已创建批量升级任务: %d 个盒子升级到版本 %s (升级包ID: %d)", len(boxIDs), upgradePackage.Version, upgradePackageID)
	return batchTask, nil
}

// ExecuteBatchUpgrade 执行批量升级任务
func (s *UpgradeService) ExecuteBatchUpgrade(batchTaskID uint) error {
	ctx := context.Background()

	// 获取批量任务中的所有子任务
	tasks, err := s.repoManager.Upgrade().FindByBatchID(ctx, batchTaskID)
	if err != nil {
		return fmt.Errorf("查找批量任务的子任务失败: %w", err)
	}

	if len(tasks) == 0 {
		return fmt.Errorf("批量任务 %d 没有子任务", batchTaskID)
	}

	// 启动批量任务
	batchTask, err := s.repoManager.Upgrade().GetBatchUpgrade(ctx, batchTaskID)
	if err != nil {
		return fmt.Errorf("查找批量任务失败: %w", err)
	}

	batchTask.Start()
	// 这里需要实现BatchUpgrade的Update方法，暂时跳过
	log.Printf("批量升级任务 %d 已开始执行", batchTaskID)

	// 执行所有子任务
	var executedCount int
	for _, task := range tasks {
		if task.Status == models.UpgradeStatusPending {
			if err := s.ExecuteUpgrade(task.ID); err != nil {
				log.Printf("执行升级任务 %d 失败: %v", task.ID, err)
				continue
			}
			executedCount++
			log.Printf("已启动升级任务 %d (盒子 %d)", task.ID, task.BoxID)
		}
	}

	if executedCount == 0 {
		return fmt.Errorf("没有可执行的升级任务")
	}

	log.Printf("批量升级任务 %d 已启动 %d 个子任务", batchTaskID, executedCount)
	return nil
}

// ExecuteUpgrade 执行升级
func (s *UpgradeService) ExecuteUpgrade(taskID uint) error {
	ctx := context.Background()

	task, err := s.repoManager.Upgrade().GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("查找升级任务失败: %w", err)
	}

	if !task.CanCancel() {
		return fmt.Errorf("任务状态不允许执行: %s", task.Status)
	}

	// 标记任务开始
	task.Start()
	if err := s.repoManager.Upgrade().Update(ctx, task); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 启动升级协程
	go s.performUpgrade(task)

	return nil
}

// performUpgrade 执行实际的升级过程
func (s *UpgradeService) performUpgrade(task *models.UpgradeTask) {
	ctx := context.Background()

	// 更新进度: 开始升级
	s.updateProgress(task, 10, "开始升级...")

	// 获取升级包信息
	upgradePackage, err := s.repoManager.UpgradePackage().GetByID(ctx, task.UpgradePackageID)
	if err != nil {
		s.failUpgrade(task, fmt.Sprintf("获取升级包信息失败: %v", err))
		return
	}
	if upgradePackage == nil {
		s.failUpgrade(task, "升级包不存在")
		return
	}

	// 获取后台程序文件
	backendFile := upgradePackage.GetFileByType(models.FileTypeBackendProgram)
	if backendFile == nil {
		s.failUpgrade(task, "升级包中不包含后台程序文件")
		return
	}

	// 准备升级数据
	upgradeData := map[string]interface{}{
		"version":      task.VersionTo,
		"program_file": backendFile.Path, // 使用升级包中的文件路径
		"program_size": backendFile.Size,
		"checksum":     backendFile.Checksum,
		"force":        task.Force,
	}

	// 调用盒子升级API
	s.updateProgress(task, 30, "上传升级文件...")
	resp, err := s.proxyService.UpdateSystem(task.BoxID, upgradeData)
	if err != nil {
		s.failUpgrade(task, fmt.Sprintf("调用升级API失败: %v", err))
		return
	}

	if !resp.Success {
		s.failUpgrade(task, fmt.Sprintf("升级API返回错误: %s", resp.Error))
		return
	}

	// 文件上传成功，认为升级完成
	s.updateProgress(task, 90, "升级文件上传成功，升级完成")
	log.Printf("盒子 %d 升级文件上传成功，升级任务完成", task.BoxID)

	// 完成升级
	s.updateProgress(task, 100, "升级完成")
	task.Complete()
	if err := s.repoManager.Upgrade().Update(ctx, task); err != nil {
		log.Printf("保存升级任务完成状态失败: %v", err)
	}

	// 更新升级包使用统计
	upgradePackage.MarkAsUpgraded()
	if err := s.repoManager.UpgradePackage().Update(ctx, upgradePackage); err != nil {
		log.Printf("更新升级包使用统计失败: %v", err)
	}

	// 更新版本记录
	s.recordVersion(task)

	// 如果是批量任务的一部分，更新批量任务进度
	s.updateBatchTaskProgress(task)

	log.Printf("盒子 %s 升级完成: %s -> %s", task.Box.Name, task.VersionFrom, task.VersionTo)
}

// updateProgress 更新升级进度
func (s *UpgradeService) updateProgress(task *models.UpgradeTask, progress int, message string) {
	task.UpdateProgress(progress)
	ctx := context.Background()
	if err := s.repoManager.Upgrade().UpdateProgress(ctx, task.ID, progress); err != nil {
		log.Printf("更新升级进度失败: %v", err)
	}
	log.Printf("升级任务 %d 进度: %d%% - %s", task.ID, progress, message)
}

// failUpgrade 标记升级失败
func (s *UpgradeService) failUpgrade(task *models.UpgradeTask, errorMsg string) {
	errorDetail := models.ErrorDetail{
		Code:      "UPGRADE_FAILED",
		Message:   errorMsg,
		Timestamp: time.Now(),
	}

	task.Fail(errorMsg, errorDetail)
	ctx := context.Background()
	if err := s.repoManager.Upgrade().Update(ctx, task); err != nil {
		log.Printf("保存升级失败状态失败: %v", err)
	}

	// 更新批量任务进度
	s.updateBatchTaskProgress(task)

	log.Printf("盒子 %s 升级失败: %s", task.Box.Name, errorMsg)
}

// recordVersion 记录版本信息
func (s *UpgradeService) recordVersion(task *models.UpgradeTask) {
	ctx := context.Background()
	if err := s.repoManager.Upgrade().RecordVersion(ctx, task); err != nil {
		log.Printf("记录版本信息失败: %v", err)
	}
}

// updateBatchTaskProgress 更新批量任务进度
func (s *UpgradeService) updateBatchTaskProgress(task *models.UpgradeTask) {
	if task.BatchUpgradeTaskID == nil {
		return
	}

	ctx := context.Background()

	// 获取批量任务中的所有任务
	tasks, err := s.repoManager.Upgrade().FindByBatchID(ctx, *task.BatchUpgradeTaskID)
	if err != nil {
		log.Printf("查找批量任务的子任务失败: %v", err)
		return
	}

	// 统计完成和失败的任务
	var completed, failed int
	for _, t := range tasks {
		if t.Status == models.UpgradeStatusCompleted {
			completed++
		} else if t.Status == models.UpgradeStatusFailed {
			failed++
		}
	}

	// 获取批量任务
	batchTask, err := s.repoManager.Upgrade().GetBatchUpgrade(ctx, *task.BatchUpgradeTaskID)
	if err != nil {
		log.Printf("查找批量任务失败: %v", err)
		return
	}

	// 更新批量任务进度
	batchTask.UpdateProgress(completed, failed)

	// 这里需要实现BatchUpgrade的Update方法，暂时跳过
	log.Printf("批量任务 %d 进度更新: 完成 %d, 失败 %d", *task.BatchUpgradeTaskID, completed, failed)
}

// CancelUpgradeTask 取消升级任务
func (s *UpgradeService) CancelUpgradeTask(taskID uint) error {
	ctx := context.Background()

	task, err := s.repoManager.Upgrade().GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("查找升级任务失败: %w", err)
	}

	if !task.CanCancel() {
		return fmt.Errorf("任务状态不允许取消: %s", task.Status)
	}

	task.Cancel()
	return s.repoManager.Upgrade().Update(ctx, task)
}

// RetryUpgradeTask 重试升级任务
func (s *UpgradeService) RetryUpgradeTask(taskID uint) error {
	ctx := context.Background()

	task, err := s.repoManager.Upgrade().GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("查找升级任务失败: %w", err)
	}

	if !task.CanRetry() {
		return fmt.Errorf("任务状态不允许重试: %s", task.Status)
	}

	// 重置任务状态
	task.Status = models.UpgradeStatusPending
	task.Progress = 0
	task.ErrorMessage = ""
	task.StartedAt = nil
	task.CompletedAt = nil

	if err := s.repoManager.Upgrade().Update(ctx, task); err != nil {
		return fmt.Errorf("重置任务状态失败: %w", err)
	}

	// 重新执行
	return s.ExecuteUpgrade(taskID)
}

// RollbackVersion 版本回滚
func (s *UpgradeService) RollbackVersion(boxID uint, targetUpgradePackageID uint, createdBy uint) (*models.UpgradeTask, error) {
	ctx := context.Background()

	// 获取目标升级包信息
	targetPackage, err := s.repoManager.UpgradePackage().GetByID(ctx, targetUpgradePackageID)
	if err != nil {
		return nil, fmt.Errorf("查找目标升级包失败: %w", err)
	}
	if targetPackage == nil {
		return nil, fmt.Errorf("目标升级包 %d 不存在", targetUpgradePackageID)
	}

	// 获取当前版本
	currentVersion, err := s.repoManager.Upgrade().GetCurrentVersion(ctx, boxID)
	if err != nil {
		return nil, fmt.Errorf("获取当前版本失败: %w", err)
	}

	// 创建回滚任务（实际上是一个特殊的升级任务）
	task, err := s.CreateUpgradeTask(boxID, targetUpgradePackageID, false, createdBy)
	if err != nil {
		return nil, fmt.Errorf("创建回滚任务失败: %w", err)
	}

	// 标记为回滚任务
	task.Name = fmt.Sprintf("回滚盒子版本: %s -> %s", currentVersion.Version, targetPackage.Version)
	task.RollbackVersion = currentVersion.Version
	s.repoManager.Upgrade().Update(ctx, task)

	return task, nil
}

// GetUpgradeTasks 获取升级任务列表
func (s *UpgradeService) GetUpgradeTasks(boxID *uint, status *models.UpgradeStatus, limit, offset int) ([]*models.UpgradeTask, int64, error) {
	ctx := context.Background()

	// 构建查询条件
	conditions := make(map[string]interface{})
	if boxID != nil {
		conditions["box_id"] = *boxID
	}
	if status != nil {
		conditions["status"] = *status
	}

	// 计算页码
	page := 1
	pageSize := limit
	if limit > 0 && offset >= 0 {
		page = offset/limit + 1
	}

	// 使用repository方法查询
	tasks, total, err := s.repoManager.Upgrade().FindUpgradeTasksWithOptions(ctx, conditions, page, pageSize, []string{"Box"})
	return tasks, total, err
}

// TrackProgress 跟踪升级进度（用于外部查询）
func (s *UpgradeService) TrackProgress(taskID uint) (*models.UpgradeTask, error) {
	ctx := context.Background()
	return s.repoManager.Upgrade().GetTaskWithBox(ctx, taskID)
}

// DeleteUpgradeTask 删除升级任务
func (s *UpgradeService) DeleteUpgradeTask(taskID uint) error {
	ctx := context.Background()

	// 查询任务是否存在
	task, err := s.repoManager.Upgrade().GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("升级任务不存在: %w", err)
	}

	// 检查任务状态，只能删除已完成、失败或取消的任务
	if task.Status == models.UpgradeStatusRunning {
		return fmt.Errorf("无法删除正在进行的升级任务")
	}

	// 删除任务
	return s.repoManager.Upgrade().Delete(ctx, taskID)
}
