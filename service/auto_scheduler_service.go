/*
 * @module service/auto_scheduler_service
 * @description 自动调度服务 - 基于调度策略自动分配任务到盒子
 * @architecture 服务层
 * @documentReference REQ-005: 任务管理功能
 * @stateFlow 定时触发/事件触发 -> 获取待调度任务 -> 评估策略 -> 分配任务 -> 执行部署
 * @rules 支持多种触发方式和调度策略
 * @dependencies repository, models
 */

package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"box-manage-service/models"
	"box-manage-service/repository"
)

// AutoSchedulerStatus 自动调度器状态
type AutoSchedulerStatus struct {
	IsRunning           bool                   `json:"is_running"`
	StartedAt           *time.Time             `json:"started_at,omitempty"`
	LastScheduleAt      *time.Time             `json:"last_schedule_at,omitempty"`
	TotalScheduled      int64                  `json:"total_scheduled"`
	TotalFailed         int64                  `json:"total_failed"`
	ActivePolicies      int                    `json:"active_policies"`
	PendingTasks        int                    `json:"pending_tasks"`
	AvailableBoxes      int                    `json:"available_boxes"`
	LastError           string                 `json:"last_error,omitempty"`
	ScheduleIntervalSec int                    `json:"schedule_interval_sec"`
	Statistics          map[string]interface{} `json:"statistics,omitempty"`
}

// AutoScheduleResult 自动调度结果
type AutoScheduleResult struct {
	TotalTasks     int                     `json:"total_tasks"`
	ScheduledTasks int                     `json:"scheduled_tasks"`
	FailedTasks    int                     `json:"failed_tasks"`
	SkippedTasks   int                     `json:"skipped_tasks"`
	PolicyUsed     string                  `json:"policy_used"`
	Duration       time.Duration           `json:"duration"`
	Details        []*TaskScheduleResult   `json:"details,omitempty"`
	Errors         []string                `json:"errors,omitempty"`
}

// autoSchedulerService 自动调度服务实现
type autoSchedulerService struct {
	repoManager      repository.RepositoryManager
	schedulerService TaskSchedulerService
	logService       SystemLogService

	// 运行状态
	mu        sync.RWMutex
	isRunning bool
	stopChan  chan struct{}
	wg        sync.WaitGroup

	// 统计信息
	startedAt      *time.Time
	lastScheduleAt *time.Time
	totalScheduled int64
	totalFailed    int64
	lastError      string

	// 配置
	defaultInterval time.Duration
}

// NewAutoSchedulerService 创建自动调度服务实例
func NewAutoSchedulerService(
	repoManager repository.RepositoryManager,
	schedulerService TaskSchedulerService,
	logService SystemLogService,
) AutoSchedulerService {
	return &autoSchedulerService{
		repoManager:      repoManager,
		schedulerService: schedulerService,
		logService:       logService,
		defaultInterval:  60 * time.Second, // 默认60秒检查一次
	}
}

// Start 启动自动调度器
func (s *autoSchedulerService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return fmt.Errorf("自动调度器已在运行")
	}

	s.isRunning = true
	s.stopChan = make(chan struct{})
	now := time.Now()
	s.startedAt = &now

	// 记录启动日志
	if s.logService != nil {
		s.logService.Info("auto_scheduler_service", "自动调度器启动", "自动调度器已启动")
	}

	// 启动调度循环
	s.wg.Add(1)
	go s.scheduleLoop(ctx)

	log.Printf("[AutoSchedulerService] 自动调度器已启动")
	return nil
}

// Stop 停止自动调度器
func (s *autoSchedulerService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return fmt.Errorf("自动调度器未运行")
	}

	close(s.stopChan)
	s.wg.Wait()
	s.isRunning = false

	// 记录停止日志
	if s.logService != nil {
		s.logService.Info("auto_scheduler_service", "自动调度器停止", "自动调度器已停止")
	}

	log.Printf("[AutoSchedulerService] 自动调度器已停止")
	return nil
}

// IsRunning 检查自动调度器是否正在运行
func (s *autoSchedulerService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetStatus 获取自动调度器状态
func (s *autoSchedulerService) GetStatus(ctx context.Context) *AutoSchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &AutoSchedulerStatus{
		IsRunning:           s.isRunning,
		StartedAt:           s.startedAt,
		LastScheduleAt:      s.lastScheduleAt,
		TotalScheduled:      s.totalScheduled,
		TotalFailed:         s.totalFailed,
		LastError:           s.lastError,
		ScheduleIntervalSec: int(s.defaultInterval.Seconds()),
	}

	// 获取活跃策略数量
	if policies, err := s.repoManager.SchedulePolicy().FindEnabled(ctx); err == nil {
		status.ActivePolicies = len(policies)
	}

	// 获取待调度任务数量
	if tasks, err := s.repoManager.Task().FindAutoScheduleTasks(ctx, 1000); err == nil {
		status.PendingTasks = len(tasks)
	}

	// 获取可用盒子数量
	if boxes, err := s.repoManager.Box().FindOnlineBoxes(ctx); err == nil {
		status.AvailableBoxes = len(boxes)
	}

	return status
}

// TriggerSchedule 手动触发一次调度
func (s *autoSchedulerService) TriggerSchedule(ctx context.Context) (*AutoScheduleResult, error) {
	log.Printf("[AutoSchedulerService] 手动触发调度")
	return s.executeSchedule(ctx)
}

// TriggerScheduleWithPolicy 使用指定策略触发调度
func (s *autoSchedulerService) TriggerScheduleWithPolicy(ctx context.Context, policyID uint) (*AutoScheduleResult, error) {
	log.Printf("[AutoSchedulerService] 使用策略 %d 触发调度", policyID)

	// 获取策略
	policy, err := s.repoManager.SchedulePolicy().GetByID(ctx, policyID)
	if err != nil {
		return nil, fmt.Errorf("获取调度策略失败: %w", err)
	}

	return s.executeScheduleWithPolicy(ctx, policy)
}

// OnNewTask 新任务创建事件处理
func (s *autoSchedulerService) OnNewTask(ctx context.Context, taskID uint) error {
	if !s.isRunning {
		return nil
	}

	// 检查是否有策略配置了新任务触发
	policies, err := s.repoManager.SchedulePolicy().FindEnabled(ctx)
	if err != nil {
		return err
	}

	for _, policy := range policies {
		if policy.TriggerConditions.OnNewTask {
			log.Printf("[AutoSchedulerService] 新任务 %d 触发调度", taskID)
			go s.scheduleSpecificTask(ctx, taskID, policy)
			break
		}
	}

	return nil
}

// OnBoxOnline 盒子上线事件处理
func (s *autoSchedulerService) OnBoxOnline(ctx context.Context, boxID uint) error {
	if !s.isRunning {
		return nil
	}

	// 检查是否有策略配置了盒子上线触发
	policies, err := s.repoManager.SchedulePolicy().FindEnabled(ctx)
	if err != nil {
		return err
	}

	for _, policy := range policies {
		if policy.TriggerConditions.OnBoxOnline {
			log.Printf("[AutoSchedulerService] 盒子 %d 上线触发调度", boxID)
			go func() {
				_, _ = s.executeScheduleWithPolicy(ctx, policy)
			}()
			break
		}
	}

	return nil
}

// scheduleLoop 调度循环
func (s *autoSchedulerService) scheduleLoop(ctx context.Context) {
	defer s.wg.Done()

	// 获取调度间隔
	interval := s.getScheduleInterval(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[AutoSchedulerService] 调度循环已启动，间隔: %v", interval)

	for {
		select {
		case <-ticker.C:
			// 执行调度
			result, err := s.executeSchedule(ctx)
			if err != nil {
				s.mu.Lock()
				s.lastError = err.Error()
				s.mu.Unlock()
				log.Printf("[AutoSchedulerService] 调度执行失败: %v", err)
			} else {
				log.Printf("[AutoSchedulerService] 调度完成 - 总任务: %d, 已调度: %d, 失败: %d",
					result.TotalTasks, result.ScheduledTasks, result.FailedTasks)
			}

			// 更新调度间隔（可能策略配置有变化）
			newInterval := s.getScheduleInterval(ctx)
			if newInterval != interval {
				ticker.Reset(newInterval)
				interval = newInterval
				log.Printf("[AutoSchedulerService] 调度间隔已更新为: %v", interval)
			}

		case <-s.stopChan:
			log.Printf("[AutoSchedulerService] 调度循环已停止")
			return

		case <-ctx.Done():
			log.Printf("[AutoSchedulerService] 上下文取消，调度循环停止")
			return
		}
	}
}

// getScheduleInterval 获取调度间隔
func (s *autoSchedulerService) getScheduleInterval(ctx context.Context) time.Duration {
	// 获取优先级最高的启用策略
	policy, err := s.repoManager.SchedulePolicy().GetDefaultPolicy(ctx)
	if err != nil || policy == nil {
		return s.defaultInterval
	}

	if policy.TriggerConditions.IntervalSeconds > 0 {
		return time.Duration(policy.TriggerConditions.IntervalSeconds) * time.Second
	}

	return s.defaultInterval
}

// executeSchedule 执行调度
func (s *autoSchedulerService) executeSchedule(ctx context.Context) (*AutoScheduleResult, error) {
	startTime := time.Now()

	// 获取默认策略
	policy, err := s.repoManager.SchedulePolicy().GetDefaultPolicy(ctx)
	if err != nil {
		// 如果没有配置策略，使用默认行为
		log.Printf("[AutoSchedulerService] 未找到启用的调度策略，使用默认调度")
		return s.executeDefaultSchedule(ctx, startTime)
	}

	return s.executeScheduleWithPolicy(ctx, policy)
}

// executeDefaultSchedule 执行默认调度（无策略配置时）
func (s *autoSchedulerService) executeDefaultSchedule(ctx context.Context, startTime time.Time) (*AutoScheduleResult, error) {
	result := &AutoScheduleResult{
		PolicyUsed: "default",
		Details:    make([]*TaskScheduleResult, 0),
		Errors:     make([]string, 0),
	}

	// 获取所有待调度任务
	tasks, err := s.repoManager.Task().FindAutoScheduleTasks(ctx, 50)
	if err != nil {
		return nil, fmt.Errorf("获取待调度任务失败: %w", err)
	}

	result.TotalTasks = len(tasks)

	if len(tasks) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// 使用现有的调度服务进行调度
	scheduleResult, err := s.schedulerService.ScheduleAutoTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("调度任务失败: %w", err)
	}

	result.ScheduledTasks = scheduleResult.ScheduledTasks
	result.FailedTasks = scheduleResult.FailedTasks
	result.Details = scheduleResult.TaskResults
	result.Duration = time.Since(startTime)

	// 更新统计
	s.mu.Lock()
	now := time.Now()
	s.lastScheduleAt = &now
	s.totalScheduled += int64(result.ScheduledTasks)
	s.totalFailed += int64(result.FailedTasks)
	s.mu.Unlock()

	return result, nil
}

// executeScheduleWithPolicy 使用指定策略执行调度
func (s *autoSchedulerService) executeScheduleWithPolicy(ctx context.Context, policy *models.SchedulePolicy) (*AutoScheduleResult, error) {
	startTime := time.Now()

	result := &AutoScheduleResult{
		PolicyUsed: policy.Name,
		Details:    make([]*TaskScheduleResult, 0),
		Errors:     make([]string, 0),
	}

	// 获取所有待调度任务
	tasks, err := s.repoManager.Task().FindAutoScheduleTasks(ctx, 50)
	if err != nil {
		return nil, fmt.Errorf("获取待调度任务失败: %w", err)
	}

	result.TotalTasks = len(tasks)

	if len(tasks) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// 获取所有在线盒子
	boxes, err := s.repoManager.Box().FindOnlineBoxes(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取在线盒子失败: %w", err)
	}

	if len(boxes) == 0 {
		result.Duration = time.Since(startTime)
		result.Errors = append(result.Errors, "没有可用的在线盒子")
		return result, nil
	}

	// 根据策略类型执行不同的调度算法
	switch policy.PolicyType {
	case models.SchedulePolicyTypePriority:
		s.scheduleByPriority(ctx, tasks, boxes, policy, result)
	case models.SchedulePolicyTypeLoadBalance:
		s.scheduleByLoadBalance(ctx, tasks, boxes, policy, result)
	case models.SchedulePolicyTypeResourceMatch:
		s.scheduleByResourceMatch(ctx, tasks, boxes, policy, result)
	default:
		// 使用默认调度
		scheduleResult, _ := s.schedulerService.ScheduleAutoTasks(ctx)
		if scheduleResult != nil {
			result.ScheduledTasks = scheduleResult.ScheduledTasks
			result.FailedTasks = scheduleResult.FailedTasks
			result.Details = scheduleResult.TaskResults
		}
	}

	result.Duration = time.Since(startTime)

	// 更新统计
	s.mu.Lock()
	now := time.Now()
	s.lastScheduleAt = &now
	s.totalScheduled += int64(result.ScheduledTasks)
	s.totalFailed += int64(result.FailedTasks)
	s.mu.Unlock()

	// 记录日志
	if s.logService != nil {
		s.logService.Info("auto_scheduler_service", "调度完成",
			fmt.Sprintf("策略: %s, 总任务: %d, 已调度: %d, 失败: %d",
				policy.Name, result.TotalTasks, result.ScheduledTasks, result.FailedTasks))
	}

	return result, nil
}

// scheduleByPriority 按优先级调度
func (s *autoSchedulerService) scheduleByPriority(ctx context.Context, tasks []*models.Task, boxes []*models.Box, policy *models.SchedulePolicy, result *AutoScheduleResult) {
	// 按优先级排序任务（高优先级在前）
	sortedTasks := make([]*models.Task, len(tasks))
	copy(sortedTasks, tasks)

	// 简单的冒泡排序，按优先级降序
	for i := 0; i < len(sortedTasks); i++ {
		for j := i + 1; j < len(sortedTasks); j++ {
			if sortedTasks[j].Priority > sortedTasks[i].Priority {
				sortedTasks[i], sortedTasks[j] = sortedTasks[j], sortedTasks[i]
			}
		}
	}

	// 调度每个任务
	for _, task := range sortedTasks {
		taskResult, err := s.schedulerService.ScheduleTask(ctx, task.ID)
		if err != nil {
			result.FailedTasks++
			result.Errors = append(result.Errors, fmt.Sprintf("任务 %d 调度失败: %v", task.ID, err))
		} else if taskResult.Success {
			result.ScheduledTasks++
		} else {
			result.SkippedTasks++
		}
		result.Details = append(result.Details, taskResult)
	}
}

// scheduleByLoadBalance 按负载均衡调度
func (s *autoSchedulerService) scheduleByLoadBalance(ctx context.Context, tasks []*models.Task, boxes []*models.Box, policy *models.SchedulePolicy, result *AutoScheduleResult) {
	// 构建盒子负载映射
	boxTaskCount := make(map[uint]int)
	for _, box := range boxes {
		// 获取盒子当前任务数
		activeTasks, _ := s.repoManager.Task().GetActiveTasksByBoxID(ctx, box.ID)
		boxTaskCount[box.ID] = len(activeTasks)
	}

	// 为每个任务选择负载最低的盒子
	for _, task := range tasks {
		// 找到负载最低且满足条件的盒子
		var bestBox *models.Box
		minLoad := -1

		for _, box := range boxes {
			currentLoad := boxTaskCount[box.ID]

			// 检查是否超过最大任务数
			if policy.ScheduleRules.MaxTasksPerBox > 0 && currentLoad >= policy.ScheduleRules.MaxTasksPerBox {
				continue
			}

			// 检查资源阈值
			if !policy.CanScheduleToBox(
				box.Resources.CPUUsedPercent,
				box.Resources.MemoryUsedPercent,
				float64(box.Resources.TPUUsed),
				currentLoad,
			) {
				continue
			}

			// 检查标签兼容性
			if !task.IsCompatibleWithBox(box) {
				continue
			}

			if minLoad < 0 || currentLoad < minLoad {
				minLoad = currentLoad
				bestBox = box
			}
		}

		if bestBox != nil {
			// 分配任务到盒子（使用 AssignToBox 正确更新调度状态）
			task.AssignToBox(bestBox.ID)
			if err := s.repoManager.Task().Update(ctx, task); err != nil {
				result.FailedTasks++
				result.Errors = append(result.Errors, fmt.Sprintf("任务 %d 分配失败: %v", task.ID, err))
			} else {
				result.ScheduledTasks++
				boxTaskCount[bestBox.ID]++
				result.Details = append(result.Details, &TaskScheduleResult{
					TaskID:   task.ID,
					TaskName: task.Name,
					BoxID:    &bestBox.ID,
					BoxName:  bestBox.Name,
					Success:  true,
					Reason:   "负载均衡调度成功",
				})
			}
		} else {
			result.SkippedTasks++
			result.Details = append(result.Details, &TaskScheduleResult{
				TaskID:   task.ID,
				TaskName: task.Name,
				Success:  false,
				Reason:   "没有可用的盒子满足负载均衡条件",
			})
		}
	}
}

// scheduleByResourceMatch 按资源匹配调度
func (s *autoSchedulerService) scheduleByResourceMatch(ctx context.Context, tasks []*models.Task, boxes []*models.Box, policy *models.SchedulePolicy, result *AutoScheduleResult) {
	// 为每个任务找到最匹配的盒子
	for _, task := range tasks {
		var bestBox *models.Box
		var bestScore float64 = -1

		for _, box := range boxes {
			// 计算匹配分数
			score := s.calculateMatchScore(task, box, policy)

			// 检查是否满足最小匹配分数
			if policy.ScheduleRules.MinMatchScore > 0 && score < policy.ScheduleRules.MinMatchScore {
				continue
			}

			if score > bestScore {
				bestScore = score
				bestBox = box
			}
		}

		if bestBox != nil {
			// 分配任务到盒子（使用 AssignToBox 正确更新调度状态）
			task.AssignToBox(bestBox.ID)
			if err := s.repoManager.Task().Update(ctx, task); err != nil {
				result.FailedTasks++
				result.Errors = append(result.Errors, fmt.Sprintf("任务 %d 分配失败: %v", task.ID, err))
			} else {
				result.ScheduledTasks++
				result.Details = append(result.Details, &TaskScheduleResult{
					TaskID:   task.ID,
					TaskName: task.Name,
					BoxID:    &bestBox.ID,
					BoxName:  bestBox.Name,
					Score:    bestScore,
					Success:  true,
					Reason:   fmt.Sprintf("资源匹配调度成功，匹配分数: %.2f", bestScore),
				})
			}
		} else {
			result.SkippedTasks++
			result.Details = append(result.Details, &TaskScheduleResult{
				TaskID:   task.ID,
				TaskName: task.Name,
				Success:  false,
				Reason:   "没有盒子满足资源匹配条件",
			})
		}
	}
}

// calculateMatchScore 计算任务与盒子的匹配分数
func (s *autoSchedulerService) calculateMatchScore(task *models.Task, box *models.Box, policy *models.SchedulePolicy) float64 {
	var score float64 = 0

	// 盒子在线检查
	if !box.IsOnline() {
		return 0
	}

	// 基础分数
	score = 50.0

	// 标签匹配加分
	if task.IsCompatibleWithBox(box) {
		affinityTags := task.GetAffinityTags()
		boxTags := box.GetTags()
		matchCount := 0
		for _, tag := range affinityTags {
			for _, boxTag := range boxTags {
				if tag == boxTag {
					matchCount++
					break
				}
			}
		}
		if len(affinityTags) > 0 {
			score += float64(matchCount) / float64(len(affinityTags)) * 30.0
		} else {
			score += 10.0 // 无标签要求，小幅加分
		}
	} else if policy.ScheduleRules.RequireTagMatch {
		return 0 // 要求标签匹配但不匹配
	}

	// 资源可用性加分
	cpuAvailable := 100.0 - box.Resources.CPUUsedPercent
	memAvailable := 100.0 - box.Resources.MemoryUsedPercent
	score += (cpuAvailable + memAvailable) / 200.0 * 20.0

	return score
}

// scheduleSpecificTask 调度特定任务
func (s *autoSchedulerService) scheduleSpecificTask(ctx context.Context, taskID uint, policy *models.SchedulePolicy) {
	// 先检查任务是否已被调度，防止重复调度
	task, err := s.repoManager.Task().GetByID(ctx, taskID)
	if err != nil {
		log.Printf("[AutoSchedulerService] 获取任务 %d 失败: %v", taskID, err)
		return
	}
	
	// 如果任务已分配到盒子，跳过调度
	if task.IsAssigned() {
		log.Printf("[AutoSchedulerService] 任务 %d 已分配到盒子，跳过调度", taskID)
		return
	}
	
	// 如果任务没有启用自动调度，跳过
	if !task.AutoSchedule {
		log.Printf("[AutoSchedulerService] 任务 %d 未启用自动调度，跳过", taskID)
		return
	}

	taskResult, err := s.schedulerService.ScheduleTask(ctx, taskID)
	if err != nil {
		log.Printf("[AutoSchedulerService] 调度任务 %d 失败: %v", taskID, err)
		s.mu.Lock()
		s.totalFailed++
		s.mu.Unlock()
	} else if taskResult.Success {
		log.Printf("[AutoSchedulerService] 调度任务 %d 成功", taskID)
		s.mu.Lock()
		s.totalScheduled++
		s.mu.Unlock()
	}
}

// SetDefaultInterval 设置默认调度间隔
func (s *autoSchedulerService) SetDefaultInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultInterval = interval
}

