/*
 * @module service/task_scheduler_service
 * @description 任务调度服务 - 支持标签亲和性和自动调度
 * @architecture 服务层
 * @documentReference REQ-005: 任务管理功能
 * @stateFlow 任务待调度 -> 盒子匹配 -> 任务分配 -> 任务下发
 * @rules 根据标签亲和性和资源需求将任务调度到最合适的盒子
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

// taskSchedulerService 任务调度服务实现
type taskSchedulerService struct {
	taskRepo          repository.TaskRepository
	boxRepo           repository.BoxRepository
	deploymentService TaskDeploymentService
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

	// 检查任务是否已经分配到盒子
	if task.BoxID != nil {
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

	// 更新任务分配
	task.BoxID = &bestBox.BoxID
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
	score := &BoxScore{
		BoxID:    box.ID,
		BoxName:  box.Name,
		IsOnline: box.IsOnline(),
		Reasons:  []string{},
	}

	// 基础检查：盒子必须在线
	if !box.IsOnline() {
		score.Reasons = append(score.Reasons, "盒子不在线")
		return nil // 不在线的盒子直接排除
	}

	// 检查容量（简化实现）
	activeTasks, err := s.getActiveTasksCount(box.ID)
	if err != nil || activeTasks >= 5 { // 假设每个盒子最多同时运行5个任务
		score.HasCapacity = false
		score.Reasons = append(score.Reasons, fmt.Sprintf("容量不足 (活跃任务: %d)", activeTasks))
	} else {
		score.HasCapacity = true
		score.Score += 20.0 // 有容量加分
		score.Reasons = append(score.Reasons, "有足够容量")
	}

	// 标签亲和性评分
	if task.IsCompatibleWithBox(box) {
		affinityTags := task.GetAffinityTags()
		boxTags := box.GetTags()

		matchCount := s.countTagMatches(affinityTags, boxTags)
		score.TagMatches = matchCount

		if matchCount > 0 {
			score.Score += float64(matchCount) * 15.0 // 每个匹配的标签加15分
			score.Reasons = append(score.Reasons, fmt.Sprintf("标签匹配度: %d", matchCount))
		} else if len(affinityTags) == 0 {
			score.Score += 5.0 // 无亲和性要求，小幅加分
			score.Reasons = append(score.Reasons, "无标签限制")
		}
	} else {
		score.Reasons = append(score.Reasons, "标签不匹配")
		return nil // 标签不兼容的盒子直接排除
	}

	// 负载均衡评分（活跃任务越少分数越高）
	loadScore := 10.0 - float64(activeTasks)*2.0
	if loadScore > 0 {
		score.Score += loadScore
		score.Reasons = append(score.Reasons, "负载适中")
	}

	// 硬件兼容性评分（简化实现）
	if len(box.Meta.SupportedHardware) > 0 {
		score.Score += 5.0
		score.Reasons = append(score.Reasons, "支持多种硬件")
	}

	return score
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

// getActiveTasksCount 获取盒子上活跃任务数量（简化实现）
func (s *taskSchedulerService) getActiveTasksCount(boxID uint) (int, error) {
	// 这里应该查询数据库获取实际的活跃任务数量
	// 简化实现：返回固定值
	return 2, nil // 假设每个盒子有2个活跃任务
}
