/*
 * @module service/task_scheduler_service
 * @description 任务调度服务 - 支持标签亲和性、调度策略和自动调度
 * @architecture 服务层
 * @documentReference REQ-005: 任务管理功能
 * @stateFlow 任务待调度 -> 策略评估 -> 盒子匹配 -> 任务分配 -> 任务下发
 * @rules 根据调度策略、标签亲和性和资源需求将任务调度到最合适的盒子
 * @dependencies repository, models
 * @refs task_req.md
 */

package service

import (
	"context"
	"fmt"
	"log"
	"sort"

	"box-manage-service/models"
	"box-manage-service/repository"
)

// ScoreWeights 评分权重配置
type ScoreWeights struct {
	CapacityWeight       float64 `json:"capacity_weight"`        // 容量权重
	TagMatchWeight       float64 `json:"tag_match_weight"`       // 标签匹配权重
	LoadBalanceWeight    float64 `json:"load_balance_weight"`    // 负载均衡权重
	ResourceWeight       float64 `json:"resource_weight"`        // 资源权重
	HardwareWeight       float64 `json:"hardware_weight"`        // 硬件兼容性权重
	PriorityWeight       float64 `json:"priority_weight"`        // 优先级权重
}

// DefaultScoreWeights 默认评分权重
func DefaultScoreWeights() *ScoreWeights {
	return &ScoreWeights{
		CapacityWeight:    20.0,
		TagMatchWeight:    15.0,
		LoadBalanceWeight: 10.0,
		ResourceWeight:    25.0,
		HardwareWeight:    5.0,
		PriorityWeight:    10.0,
	}
}

// taskSchedulerService 任务调度服务实现
type taskSchedulerService struct {
	taskRepo          repository.TaskRepository
	boxRepo           repository.BoxRepository
	policyRepo        repository.SchedulePolicyRepository
	deploymentService TaskDeploymentService
	scoreWeights      *ScoreWeights
}

// NewTaskSchedulerService 创建任务调度服务实例
func NewTaskSchedulerService(
	taskRepo repository.TaskRepository,
	boxRepo repository.BoxRepository,
	deploymentService TaskDeploymentService,
) TaskSchedulerService {
	return &taskSchedulerService{
		taskRepo:          taskRepo,
		boxRepo:           boxRepo,
		deploymentService: deploymentService,
		scoreWeights:      DefaultScoreWeights(),
	}
}

// NewTaskSchedulerServiceWithPolicy 创建带策略支持的任务调度服务实例
func NewTaskSchedulerServiceWithPolicy(
	taskRepo repository.TaskRepository,
	boxRepo repository.BoxRepository,
	policyRepo repository.SchedulePolicyRepository,
	deploymentService TaskDeploymentService,
) TaskSchedulerService {
	return &taskSchedulerService{
		taskRepo:          taskRepo,
		boxRepo:           boxRepo,
		policyRepo:        policyRepo,
		deploymentService: deploymentService,
		scoreWeights:      DefaultScoreWeights(),
	}
}

// ScheduleAutoTasks 调度所有启用自动调度的任务
func (s *taskSchedulerService) ScheduleAutoTasks(ctx context.Context) (*ScheduleResult, error) {
	log.Printf("开始自动调度任务")

	// 获取所有启用自动调度的待执行任务
	tasks, err := s.taskRepo.FindAutoScheduleTasks(ctx, 50) // 限制一次最多调度50个任务
	if err != nil {
		return nil, fmt.Errorf("获取自动调度任务失败: %w", err)
	}

	result := &ScheduleResult{
		TotalTasks:  len(tasks),
		TaskResults: make([]*TaskScheduleResult, 0, len(tasks)),
		Summary:     make(map[string]int),
	}

	log.Printf("找到 %d 个待自动调度的任务", len(tasks))

	// 逐个调度任务
	for _, task := range tasks {
		taskResult, err := s.ScheduleTask(ctx, task.ID)
		if err != nil {
			log.Printf("调度任务 %s 失败: %v", task.Name, err)
			taskResult = &TaskScheduleResult{
				TaskID:   task.ID,
				TaskName: task.Name,
				Success:  false,
				Reason:   fmt.Sprintf("调度失败: %v", err),
			}
		}

		result.TaskResults = append(result.TaskResults, taskResult)

		if taskResult.Success {
			result.ScheduledTasks++
			result.Summary["scheduled"]++
		} else {
			result.FailedTasks++
			result.Summary["failed"]++
		}
	}

	log.Printf("自动调度完成: 总任务数=%d, 成功调度=%d, 失败=%d",
		result.TotalTasks, result.ScheduledTasks, result.FailedTasks)

	return result, nil
}

// ScheduleTask 调度单个任务到最适合的盒子
func (s *taskSchedulerService) ScheduleTask(ctx context.Context, taskID uint) (*TaskScheduleResult, error) {
	// 获取任务信息
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	result := &TaskScheduleResult{
		TaskID:   taskID,
		TaskName: task.Name,
		Success:  false,
	}

	// 检查任务是否已经分配到盒子（使用新的调度状态判断）
	if task.IsAssigned() {
		result.Success = true
		result.BoxID = task.BoxID
		result.Reason = "任务已分配到盒子"

		// 获取盒子名称
		if box, err := s.boxRepo.GetByID(ctx, *task.BoxID); err == nil {
			result.BoxName = box.Name
		}

		return result, nil
	}

	// 查找兼容的盒子
	boxScores, err := s.FindCompatibleBoxes(ctx, taskID)
	if err != nil {
		result.Reason = fmt.Sprintf("查找兼容盒子失败: %v", err)
		return result, nil
	}

	result.Candidates = boxScores

	if len(boxScores) == 0 {
		result.Reason = "没有找到兼容的盒子"
		return result, nil
	}

	// 选择评分最高的盒子
	bestBox := boxScores[0]
	result.BoxID = &bestBox.BoxID
	result.BoxName = bestBox.BoxName
	result.Score = bestBox.Score

	log.Printf("为任务 %s 选择盒子 %s (评分: %.2f)", task.Name, bestBox.BoxName, bestBox.Score)

	// 更新任务分配（使用新方法同时更新调度状态）
	task.AssignToBox(bestBox.BoxID)
	err = s.taskRepo.Update(ctx, task)
	if err != nil {
		result.Reason = fmt.Sprintf("更新任务分配失败: %v", err)
		return result, nil
	}

	result.Success = true
	result.Reason = fmt.Sprintf("成功分配到盒子 %s", bestBox.BoxName)

	return result, nil
}

// FindCompatibleBoxes 查找与任务兼容的盒子
func (s *taskSchedulerService) FindCompatibleBoxes(ctx context.Context, taskID uint) ([]*BoxScore, error) {
	// 获取任务信息
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	// 获取所有在线的盒子
	boxes, err := s.boxRepo.FindOnlineBoxes(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取在线盒子失败: %w", err)
	}

	var boxScores []*BoxScore

	// 为每个盒子评分
	for _, box := range boxes {
		score := s.calculateBoxScore(task, box)
		if score != nil {
			boxScores = append(boxScores, score)
		}
	}

	// 按评分降序排序
	sort.Slice(boxScores, func(i, j int) bool {
		return boxScores[i].Score > boxScores[j].Score
	})

	return boxScores, nil
}

// calculateBoxScore 计算盒子对任务的适配评分
func (s *taskSchedulerService) calculateBoxScore(task *models.Task, box *models.Box) *BoxScore {
	return s.calculateBoxScoreWithPolicy(task, box, nil)
}

// calculateBoxScoreWithPolicy 使用指定策略计算盒子评分
func (s *taskSchedulerService) calculateBoxScoreWithPolicy(task *models.Task, box *models.Box, policy *models.SchedulePolicy) *BoxScore {
	score := &BoxScore{
		BoxID:    box.ID,
		BoxName:  box.Name,
		IsOnline: box.IsOnline(),
		Reasons:  []string{},
	}

	weights := s.scoreWeights

	// 基础检查：盒子必须在线
	if !box.IsOnline() {
		score.Reasons = append(score.Reasons, "盒子不在线")
		return nil // 不在线的盒子直接排除
	}

	// 获取当前活跃任务数
	activeTasks, err := s.getActiveTasksCount(box.ID)
	if err != nil {
		activeTasks = 0
	}

	// 1. 容量评分
	maxTasks := 5 // 默认最大任务数
	if policy != nil && policy.ScheduleRules.MaxTasksPerBox > 0 {
		maxTasks = policy.ScheduleRules.MaxTasksPerBox
	}

	if activeTasks >= maxTasks {
		score.HasCapacity = false
		score.Reasons = append(score.Reasons, fmt.Sprintf("容量不足 (活跃任务: %d/%d)", activeTasks, maxTasks))
	} else {
		score.HasCapacity = true
		capacityScore := weights.CapacityWeight * (1.0 - float64(activeTasks)/float64(maxTasks))
		score.Score += capacityScore
		score.Reasons = append(score.Reasons, fmt.Sprintf("容量充足 (+%.1f)", capacityScore))
	}

	// 2. 资源阈值检查（如果配置了策略）
	if policy != nil {
		cpuUsage := box.Resources.CPUUsedPercent
		memUsage := box.Resources.MemoryUsedPercent
		tpuUsage := float64(box.Resources.TPUUsed)

		if !policy.CanScheduleToBox(cpuUsage, memUsage, tpuUsage, activeTasks) {
			score.Reasons = append(score.Reasons, "资源超过阈值")
			return nil // 资源不足直接排除
		}

		// 资源评分：资源越空闲分数越高
		cpuAvailable := 100.0 - cpuUsage
		memAvailable := 100.0 - memUsage
		resourceScore := weights.ResourceWeight * (cpuAvailable + memAvailable) / 200.0
		score.Score += resourceScore
		score.Reasons = append(score.Reasons, fmt.Sprintf("资源可用 (+%.1f)", resourceScore))
	} else {
		// 默认资源评分
		cpuAvailable := 100.0 - box.Resources.CPUUsedPercent
		memAvailable := 100.0 - box.Resources.MemoryUsedPercent
		resourceScore := weights.ResourceWeight * (cpuAvailable + memAvailable) / 200.0
		score.Score += resourceScore
	}

	// 3. 标签亲和性评分
	if task.IsCompatibleWithBox(box) {
		affinityTags := task.GetAffinityTags()
		boxTags := box.GetTags()

		matchCount := s.countTagMatches(affinityTags, boxTags)
		score.TagMatches = matchCount

		if matchCount > 0 {
			tagScore := float64(matchCount) * weights.TagMatchWeight
			score.Score += tagScore
			score.Reasons = append(score.Reasons, fmt.Sprintf("标签匹配: %d (+%.1f)", matchCount, tagScore))
		} else if len(affinityTags) == 0 {
			tagScore := weights.TagMatchWeight * 0.3 // 无亲和性要求，小幅加分
			score.Score += tagScore
			score.Reasons = append(score.Reasons, "无标签限制")
		}
	} else {
		// 检查是否要求严格标签匹配
		if policy != nil && policy.ScheduleRules.RequireTagMatch {
			score.Reasons = append(score.Reasons, "标签不匹配（策略要求）")
			return nil
		}
		score.Reasons = append(score.Reasons, "标签不匹配")
		return nil
	}

	// 4. 负载均衡评分（活跃任务越少分数越高）
	loadScore := weights.LoadBalanceWeight * (1.0 - float64(activeTasks)/float64(maxTasks))
	if loadScore > 0 {
		score.Score += loadScore
		score.Reasons = append(score.Reasons, fmt.Sprintf("负载均衡 (+%.1f)", loadScore))
	}

	// 5. 优先级评分（如果任务有优先级且策略支持）
	if policy != nil && policy.PolicyType == models.SchedulePolicyTypePriority {
		priorityWeight := policy.GetPriorityWeight(task.Priority)
		priorityScore := weights.PriorityWeight * priorityWeight / 5.0 // 归一化
		score.Score += priorityScore
		score.Reasons = append(score.Reasons, fmt.Sprintf("优先级加成 (+%.1f)", priorityScore))
	}

	// 6. 硬件兼容性评分
	if len(box.Meta.SupportedHardware) > 0 {
		score.Score += weights.HardwareWeight
		score.Reasons = append(score.Reasons, fmt.Sprintf("硬件支持 (+%.1f)", weights.HardwareWeight))
	}

	return score
}

// FindCompatibleBoxesWithPolicy 使用策略查找兼容盒子
func (s *taskSchedulerService) FindCompatibleBoxesWithPolicy(ctx context.Context, taskID uint, policy *models.SchedulePolicy) ([]*BoxScore, error) {
	// 获取任务信息
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	// 获取所有在线的盒子
	boxes, err := s.boxRepo.FindOnlineBoxes(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取在线盒子失败: %w", err)
	}

	var boxScores []*BoxScore

	// 为每个盒子评分
	for _, box := range boxes {
		score := s.calculateBoxScoreWithPolicy(task, box, policy)
		if score != nil {
			boxScores = append(boxScores, score)
		}
	}

	// 按评分降序排序
	sort.Slice(boxScores, func(i, j int) bool {
		return boxScores[i].Score > boxScores[j].Score
	})

	return boxScores, nil
}

// SetScoreWeights 设置评分权重
func (s *taskSchedulerService) SetScoreWeights(weights *ScoreWeights) {
	if weights != nil {
		s.scoreWeights = weights
	}
}

// countTagMatches 计算标签匹配数量
func (s *taskSchedulerService) countTagMatches(taskTags, boxTags []string) int {
	taskTagSet := make(map[string]bool)
	for _, tag := range taskTags {
		taskTagSet[tag] = true
	}

	matchCount := 0
	for _, boxTag := range boxTags {
		if taskTagSet[boxTag] {
			matchCount++
		}
	}

	return matchCount
}

// getActiveTasksCount 获取盒子上活跃任务数量
func (s *taskSchedulerService) getActiveTasksCount(boxID uint) (int, error) {
	ctx := context.Background()
	tasks, err := s.taskRepo.GetActiveTasksByBoxID(ctx, boxID)
	if err != nil {
		log.Printf("获取盒子 %d 活跃任务数量失败: %v", boxID, err)
		return 0, err
	}
	return len(tasks), nil
}
