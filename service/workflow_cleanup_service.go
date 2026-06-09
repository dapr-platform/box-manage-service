/*
 * @module service/workflow_cleanup_service
 * @description 工作流实例和日志定期清理服务
 * @architecture 服务层
 * @stateFlow 读取配置 → 每日3点触发 → 清理过期实例+日志
 * @rules 保留天数通过环境变量 WORKFLOW_INSTANCE_RETENTION_DAYS / WORKFLOW_LOG_RETENTION_DAYS 配置
 */

package service

import (
	"box-manage-service/repository"
	"context"
	"log"
	"os"
	"strconv"
	"time"
)

// WorkflowCleanupService 工作流清理服务
type WorkflowCleanupService struct {
	workflowInstRepo repository.WorkflowInstanceRepository
	workflowLogRepo  repository.WorkflowLogRepository
	instanceDays     int
	logDays          int
	stopCh           chan struct{}
}

// NewWorkflowCleanupService 创建清理服务实例
func NewWorkflowCleanupService(
	workflowInstRepo repository.WorkflowInstanceRepository,
	workflowLogRepo repository.WorkflowLogRepository,
) *WorkflowCleanupService {
	return &WorkflowCleanupService{
		workflowInstRepo: workflowInstRepo,
		workflowLogRepo:  workflowLogRepo,
		instanceDays:     getEnvInt("WORKFLOW_INSTANCE_RETENTION_DAYS", 7),
		logDays:          getEnvInt("WORKFLOW_LOG_RETENTION_DAYS", 7),
		stopCh:           make(chan struct{}),
	}
}

// Start 启动每日清理调度（每天3点执行）
func (s *WorkflowCleanupService) Start(ctx context.Context) {
	log.Printf("[WorkflowCleanup] 启动清理调度: 实例保留 %d 天, 日志保留 %d 天, 每日 03:00 执行",
		s.instanceDays, s.logDays)

	// 启动时立即执行一次
	go func() {
		s.runCleanup(ctx)
	}()

	// 定时每天 3:00 执行
	go func() {
		for {
			next := s.nextRunTime(3, 0)
			timer := time.NewTimer(time.Until(next))
			select {
			case <-timer.C:
				s.runCleanup(ctx)
			case <-s.stopCh:
				timer.Stop()
				return
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	}()
}

// Stop 停止清理调度
func (s *WorkflowCleanupService) Stop() {
	close(s.stopCh)
}

func (s *WorkflowCleanupService) runCleanup(ctx context.Context) {
	log.Println("[WorkflowCleanup] 开始执行清理...")

	// 1. 清理过期工作流实例
	instCutoff := time.Now().AddDate(0, 0, -s.instanceDays)
	if count, err := s.workflowInstRepo.CleanupOldInstances(ctx, instCutoff); err != nil {
		log.Printf("[WorkflowCleanup] 清理实例失败: %v", err)
	} else {
		log.Printf("[WorkflowCleanup] 清理了 %d 个 %d 天前的工作流实例", count, s.instanceDays)
	}

	// 2. 清理过期工作流日志
	logCutoff := time.Now().AddDate(0, 0, -s.logDays)
	if count, err := s.workflowLogRepo.CleanupOldLogs(ctx, logCutoff); err != nil {
		log.Printf("[WorkflowCleanup] 清理日志失败: %v", err)
	} else {
		log.Printf("[WorkflowCleanup] 清理了 %d 条 %d 天前的工作流日志", count, s.logDays)
	}

	log.Println("[WorkflowCleanup] 清理完成")
}

func (s *WorkflowCleanupService) nextRunTime(hour, min int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if next.Before(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			return i
		}
	}
	return defaultVal
}
