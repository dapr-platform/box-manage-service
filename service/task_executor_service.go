/*
 * @module service/task_executor_service
 * @description 任务执行器服务 - 协调任务的完整执行流程
 * @architecture 服务层
 * @documentReference REQ-005: 任务管理功能
 * @stateFlow 任务接收 -> 调度 -> 部署 -> 监控 -> 完成
 * @rules 统一管理任务执行的整个生命周期
 * @dependencies repository, service
 * @refs REQ-005.md
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

// ExecutionResult 执行结果
type ExecutionResult struct {
	TaskID       uint                  `json:"task_id"`
	SessionID    string                `json:"session_id"`
	BoxID        uint                  `json:"box_id"`
	Status       ExecutionResultStatus `json:"status"`
	StartedAt    time.Time             `json:"started_at"`
	CompletedAt  *time.Time            `json:"completed_at"`
	Duration     time.Duration         `json:"duration"`
	ErrorMessage string                `json:"error_message"`
	Statistics   *ExecutionStatistics  `json:"statistics"`
	Logs         []ExecutionLog        `json:"logs"`
}

// ExecutionResultStatus 执行结果状态
type ExecutionResultStatus string

const (
	ExecutionResultStatusSuccess    ExecutionResultStatus = "success"     // 执行成功
	ExecutionResultStatusFailed     ExecutionResultStatus = "failed"      // 执行失败
	ExecutionResultStatusCancelled  ExecutionResultStatus = "cancelled"   // 被取消
	ExecutionResultStatusTimeout    ExecutionResultStatus = "timeout"     // 超时
	ExecutionResultStatusInProgress ExecutionResultStatus = "in_progress" // 进行中
)

// ExecutionSession 执行会话
type ExecutionSession struct {
	ID            string                 `json:"id"`
	TaskID        uint                   `json:"task_id"`
	BoxID         uint                   `json:"box_id"`
	Status        ExecutionSessionStatus `json:"status"`
	StartedAt     time.Time              `json:"started_at"`
	LastHeartbeat time.Time              `json:"last_heartbeat"`
	Progress      float64                `json:"progress"`
	Phase         ExecutionPhase         `json:"phase"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// ExecutionSessionStatus 执行会话状态
type ExecutionSessionStatus string

const (
	ExecutionSessionStatusInitializing ExecutionSessionStatus = "initializing" // 初始化中
	ExecutionSessionStatusRunning      ExecutionSessionStatus = "running"      // 运行中
	ExecutionSessionStatusPaused       ExecutionSessionStatus = "paused"       // 暂停
	ExecutionSessionStatusCompleted    ExecutionSessionStatus = "completed"    // 完成
	ExecutionSessionStatusFailed       ExecutionSessionStatus = "failed"       // 失败
	ExecutionSessionStatusCancelled    ExecutionSessionStatus = "cancelled"    // 取消
)

// ExecutionPhase 执行阶段
type ExecutionPhase string

const (
	ExecutionPhasePreparation ExecutionPhase = "preparation" // 准备阶段
	ExecutionPhaseValidation  ExecutionPhase = "validation"  // 验证阶段
	ExecutionPhaseModelCheck  ExecutionPhase = "model_check" // 模型检查
	ExecutionPhaseDeployment  ExecutionPhase = "deployment"  // 部署阶段
	ExecutionPhaseExecution   ExecutionPhase = "execution"   // 执行阶段
	ExecutionPhaseMonitoring  ExecutionPhase = "monitoring"  // 监控阶段
	ExecutionPhaseCleanup     ExecutionPhase = "cleanup"     // 清理阶段
	ExecutionPhaseCompleted   ExecutionPhase = "completed"   // 完成
)

// ExecutionStatus 执行状态
type ExecutionStatus struct {
	Session     *ExecutionSession   `json:"session"`
	BoxStatus   *BoxExecutionStatus `json:"box_status"`
	Performance *PerformanceMetrics `json:"performance"`
	Issues      []ExecutionIssue    `json:"issues"`
	LastUpdated time.Time           `json:"last_updated"`
}

// BoxExecutionStatus 盒子执行状态
type BoxExecutionStatus struct {
	IsOnline      bool                `json:"is_online"`
	ResourceUsage ResourceUsageStatus `json:"resource_usage"`
	TaskCount     int                 `json:"task_count"`
	LastHeartbeat time.Time           `json:"last_heartbeat"`
	Status        string              `json:"status"`
}

// ResourceUsageStatus 资源使用状态
type ResourceUsageStatus struct {
	CPU    float64 `json:"cpu"`    // CPU使用率 0-100
	Memory float64 `json:"memory"` // 内存使用率 0-100
	GPU    float64 `json:"gpu"`    // GPU使用率 0-100
	Disk   float64 `json:"disk"`   // 磁盘使用率 0-100
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	FPS             float64 `json:"fps"`              // 帧率
	Latency         float64 `json:"latency"`          // 延迟（毫秒）
	Throughput      float64 `json:"throughput"`       // 吞吐量
	ProcessedFrames int64   `json:"processed_frames"` // 已处理帧数
	TotalFrames     int64   `json:"total_frames"`     // 总帧数
	InferenceCount  int64   `json:"inference_count"`  // 推理次数
	ForwardSuccess  int64   `json:"forward_success"`  // 转发成功次数
	ForwardFailed   int64   `json:"forward_failed"`   // 转发失败次数
}

// ExecutionIssue 执行问题
type ExecutionIssue struct {
	Type      IssueType     `json:"type"`
	Severity  IssueSeverity `json:"severity"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
	Resolved  bool          `json:"resolved"`
	Solution  string        `json:"solution"`
}

// IssueSeverity 问题严重程度
type IssueSeverity string

const (
	IssueSeverityInfo     IssueSeverity = "info"     // 信息
	IssueSeverityWarning  IssueSeverity = "warning"  // 警告
	IssueSeverityError    IssueSeverity = "error"    // 错误
	IssueSeverityCritical IssueSeverity = "critical" // 严重
)

// ExecutionStatistics 执行统计
type ExecutionStatistics struct {
	TotalDuration     time.Duration                    `json:"total_duration"`
	PhaseDurations    map[ExecutionPhase]time.Duration `json:"phase_durations"`
	ResourcePeakUsage ResourceUsageStatus              `json:"resource_peak_usage"`
	Performance       PerformanceMetrics               `json:"performance"`
	ErrorCount        int                              `json:"error_count"`
	WarningCount      int                              `json:"warning_count"`
}

// ExecutionRecord 执行记录
type ExecutionRecord struct {
	ID           uint                  `json:"id"`
	TaskID       uint                  `json:"task_id"`
	SessionID    string                `json:"session_id"`
	BoxID        uint                  `json:"box_id"`
	BoxName      string                `json:"box_name"`
	Status       ExecutionResultStatus `json:"status"`
	StartedAt    time.Time             `json:"started_at"`
	CompletedAt  *time.Time            `json:"completed_at"`
	Duration     time.Duration         `json:"duration"`
	ErrorMessage string                `json:"error_message"`
	Statistics   *ExecutionStatistics  `json:"statistics"`
}

// ExecutionLog 执行日志
type ExecutionLog struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Phase     ExecutionPhase         `json:"phase"`
	Message   string                 `json:"message"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// ExecutorStatus 执行器状态
type ExecutorStatus struct {
	IsRunning            bool                         `json:"is_running"`
	ActiveSessions       int                          `json:"active_sessions"`
	TotalExecutions      int64                        `json:"total_executions"`
	SuccessfulExecutions int64                        `json:"successful_executions"`
	FailedExecutions     int64                        `json:"failed_executions"`
	AvgExecutionTime     time.Duration                `json:"avg_execution_time"`
	LastExecutionAt      *time.Time                   `json:"last_execution_at"`
	WorkerCount          int                          `json:"worker_count"`
	QueueLength          int                          `json:"queue_length"`
	Sessions             map[string]*ExecutionSession `json:"sessions"`
}

// taskExecutorService 任务执行器服务实现
type taskExecutorService struct {
	taskRepo          repository.TaskRepository
	boxRepo           repository.BoxRepository
	schedulerService  TaskSchedulerService
	deploymentService TaskDeploymentService
	dependencyService ModelDependencyService

	// 执行器状态
	sessions       map[string]*ExecutionSession
	sessionMutex   sync.RWMutex
	isRunning      bool
	executorCtx    context.Context
	executorCancel context.CancelFunc

	// 统计信息
	totalExecutions      int64
	successfulExecutions int64
	failedExecutions     int64
	executionTimes       []time.Duration
}

// NewTaskExecutorService 创建任务执行器服务实例
func NewTaskExecutorService(
	taskRepo repository.TaskRepository,
	boxRepo repository.BoxRepository,
	schedulerService TaskSchedulerService,
	deploymentService TaskDeploymentService,
	dependencyService ModelDependencyService,
) TaskExecutorService {
	return &taskExecutorService{
		taskRepo:          taskRepo,
		boxRepo:           boxRepo,
		schedulerService:  schedulerService,
		deploymentService: deploymentService,
		dependencyService: dependencyService,
		sessions:          make(map[string]*ExecutionSession),
		executionTimes:    make([]time.Duration, 0),
	}
}

// ExecuteTask 执行单个任务（完整流程）
func (s *taskExecutorService) ExecuteTask(ctx context.Context, taskID uint) (*ExecutionResult, error) {
	startTime := time.Now()

	// 增加执行计数
	s.totalExecutions++

	result := &ExecutionResult{
		TaskID:    taskID,
		StartedAt: startTime,
		Status:    ExecutionResultStatusInProgress,
		Logs:      make([]ExecutionLog, 0),
	}

	// 记录开始执行
	s.addLog(result, LogLevelInfo, ExecutionPhasePreparation, "开始执行任务")

	// 第一步：获取任务信息
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		s.failExecution(result, ExecutionPhasePreparation, fmt.Sprintf("获取任务失败: %v", err))
		return result, err
	}

	s.addLog(result, LogLevelInfo, ExecutionPhasePreparation, fmt.Sprintf("任务信息获取成功: %s", task.Name))

	// 第二步：验证任务配置
	s.addLog(result, LogLevelInfo, ExecutionPhaseValidation, "开始验证任务配置")
	if err := s.deploymentService.ValidateTaskConfig(ctx, task); err != nil {
		s.failExecution(result, ExecutionPhaseValidation, fmt.Sprintf("任务配置验证失败: %v", err))
		return result, err
	}

	// 第三步：调度任务到合适的盒子
	s.addLog(result, LogLevelInfo, ExecutionPhasePreparation, "开始任务调度")
	scheduleResult, err := s.schedulerService.ScheduleTask(ctx, taskID)
	if err != nil {
		s.failExecution(result, ExecutionPhasePreparation, fmt.Sprintf("任务调度失败: %v", err))
		return result, err
	}

	if !scheduleResult.Success {
		s.failExecution(result, ExecutionPhasePreparation, fmt.Sprintf("任务调度失败: %s", scheduleResult.Reason))
		return result, fmt.Errorf("任务调度失败: %s", scheduleResult.Reason)
	}

	if scheduleResult.BoxID != nil {
		result.BoxID = *scheduleResult.BoxID
	}
	s.addLog(result, LogLevelInfo, ExecutionPhasePreparation, fmt.Sprintf("任务调度成功，分配到盒子: %s", scheduleResult.BoxName))

	// 第四步：在指定盒子上执行任务
	if scheduleResult.BoxID != nil {
		return s.ExecuteTaskOnBox(ctx, taskID, *scheduleResult.BoxID)
	} else {
		s.failExecution(result, ExecutionPhasePreparation, "任务调度成功但未分配到盒子")
		return result, fmt.Errorf("任务调度成功但未分配到盒子")
	}
}

// ExecuteTaskOnBox 在指定盒子上执行任务
func (s *taskExecutorService) ExecuteTaskOnBox(ctx context.Context, taskID uint, boxID uint) (*ExecutionResult, error) {
	startTime := time.Now()

	result := &ExecutionResult{
		TaskID:    taskID,
		BoxID:     boxID,
		StartedAt: startTime,
		Status:    ExecutionResultStatusInProgress,
		Logs:      make([]ExecutionLog, 0),
	}

	// 第一步：检查模型依赖
	s.addLog(result, LogLevelInfo, ExecutionPhaseModelCheck, "开始检查模型依赖")
	dependencyResult, err := s.dependencyService.CheckTaskModelDependency(ctx, taskID)
	if err != nil {
		s.failExecution(result, ExecutionPhaseModelCheck, fmt.Sprintf("模型依赖检查失败: %v", err))
		return result, err
	}

	if dependencyResult.Status == DependencyStatusMissing {
		s.addLog(result, LogLevelInfo, ExecutionPhaseModelCheck, "模型未部署，但现在使用动态模型部署")
		// TODO: 实现基于OriginalModelID的动态模型部署
		s.addLog(result, LogLevelInfo, ExecutionPhaseModelCheck, "模型检查将在任务部署时动态进行")
	}

	// 第二步：部署任务到盒子
	s.addLog(result, LogLevelInfo, ExecutionPhaseDeployment, "开始部署任务到盒子")
	deployResult, err := s.deploymentService.DeployTask(ctx, taskID, boxID)
	if err != nil {
		s.failExecution(result, ExecutionPhaseDeployment, fmt.Sprintf("任务部署失败: %v", err))
		return result, err
	}

	if deployResult.Status != TaskDeploymentStatusSuccess {
		s.failExecution(result, ExecutionPhaseDeployment, fmt.Sprintf("任务部署失败: %s", deployResult.Message))
		return result, fmt.Errorf("任务部署失败: %s", deployResult.Message)
	}

	result.SessionID = deployResult.ExecutionID
	s.addLog(result, LogLevelInfo, ExecutionPhaseDeployment, "任务部署成功")

	// 第三步：开始监控任务执行
	session := &ExecutionSession{
		ID:            deployResult.ExecutionID,
		TaskID:        taskID,
		BoxID:         boxID,
		Status:        ExecutionSessionStatusRunning,
		StartedAt:     time.Now(),
		LastHeartbeat: time.Now(),
		Progress:      0,
		Phase:         ExecutionPhaseExecution,
		Metadata:      make(map[string]interface{}),
	}

	s.sessionMutex.Lock()
	s.sessions[session.ID] = session
	s.sessionMutex.Unlock()

	s.addLog(result, LogLevelInfo, ExecutionPhaseExecution, "任务开始执行，进入监控阶段")

	// 启动监控协程
	go s.monitorSession(session, result)

	// 等待任务完成或超时
	err = s.waitForCompletion(ctx, session, result)
	if err != nil {
		s.failExecution(result, ExecutionPhaseExecution, fmt.Sprintf("任务执行失败: %v", err))
		return result, err
	}

	// 任务完成
	s.completeExecution(result)
	return result, nil
}

// calculateDiskUsagePercent 计算磁盘使用百分比
func (s *taskExecutorService) calculateDiskUsagePercent(diskData []models.DiskUsage) float64 {
	if len(diskData) == 0 {
		return 0
	}

	// 计算主要磁盘的使用百分比（优先选择根目录或数据目录）
	for _, disk := range diskData {
		if disk.Path == "/" || disk.Path == "/data" {
			return disk.UsedPercent
		}
	}

	// 如果没找到主要磁盘，返回第一个磁盘的使用百分比
	return diskData[0].UsedPercent
}

// StartTaskExecution 启动任务执行
func (s *taskExecutorService) StartTaskExecution(ctx context.Context, taskID uint) (*ExecutionSession, error) {
	// 这是一个异步版本的ExecuteTask
	go func() {
		_, err := s.ExecuteTask(ctx, taskID)
		if err != nil {
			log.Printf("异步任务执行失败: %v", err)
		}
	}()

	// 返回临时会话信息
	sessionID := fmt.Sprintf("exec_%d_%d", taskID, time.Now().Unix())
	session := &ExecutionSession{
		ID:            sessionID,
		TaskID:        taskID,
		Status:        ExecutionSessionStatusInitializing,
		StartedAt:     time.Now(),
		LastHeartbeat: time.Now(),
		Progress:      0,
		Phase:         ExecutionPhasePreparation,
		Metadata:      make(map[string]interface{}),
	}

	s.sessionMutex.Lock()
	s.sessions[session.ID] = session
	s.sessionMutex.Unlock()

	return session, nil
}

// MonitorTaskExecution 监控任务执行状态
func (s *taskExecutorService) MonitorTaskExecution(ctx context.Context, sessionID string) (*ExecutionStatus, error) {
	s.sessionMutex.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("执行会话不存在: %s", sessionID)
	}

	// 获取盒子状态
	box, err := s.boxRepo.GetByID(ctx, session.BoxID)
	if err != nil {
		return nil, fmt.Errorf("获取盒子状态失败: %w", err)
	}

	boxStatus := &BoxExecutionStatus{
		IsOnline: box.IsOnline(),
		ResourceUsage: ResourceUsageStatus{
			CPU:    box.Resources.CPUUsedPercent,
			Memory: box.Resources.MemoryUsedPercent,
			GPU:    float64(box.Resources.TPUUsed), // 使用TPU作为GPU替代
			Disk:   s.calculateDiskUsagePercent(box.Resources.DiskData),
		},
		LastHeartbeat: *box.LastHeartbeat,
		Status:        string(box.Status),
	}

	// 获取任务性能指标
	taskStatus, err := s.deploymentService.GetTaskStatusFromBox(ctx, session.TaskID, session.BoxID)
	if err != nil {
		log.Printf("获取任务状态失败: %v", err)
	}

	var performance *PerformanceMetrics
	if taskStatus != nil && taskStatus.Statistics != nil {
		performance = &PerformanceMetrics{
			FPS:             taskStatus.Statistics.FPS,
			Latency:         taskStatus.Statistics.AverageLatency,
			ProcessedFrames: taskStatus.Statistics.ProcessedFrames,
			TotalFrames:     taskStatus.Statistics.TotalFrames,
			InferenceCount:  taskStatus.Statistics.InferenceCount,
			ForwardSuccess:  taskStatus.Statistics.ForwardSuccess,
			ForwardFailed:   taskStatus.Statistics.ForwardFailed,
		}
	}

	status := &ExecutionStatus{
		Session:     session,
		BoxStatus:   boxStatus,
		Performance: performance,
		Issues:      []ExecutionIssue{},
		LastUpdated: time.Now(),
	}

	return status, nil
}

// StopTaskExecution 停止任务执行
func (s *taskExecutorService) StopTaskExecution(ctx context.Context, sessionID string) error {
	s.sessionMutex.Lock()
	session, exists := s.sessions[sessionID]
	if !exists {
		s.sessionMutex.Unlock()
		return fmt.Errorf("执行会话不存在: %s", sessionID)
	}

	session.Status = ExecutionSessionStatusCancelled
	s.sessionMutex.Unlock()

	// 停止盒子上的任务
	err := s.deploymentService.StopTaskOnBox(ctx, session.TaskID, session.BoxID)
	if err != nil {
		return fmt.Errorf("停止盒子上的任务失败: %w", err)
	}

	// 清理会话
	s.sessionMutex.Lock()
	delete(s.sessions, sessionID)
	s.sessionMutex.Unlock()

	return nil
}

// GetExecutionHistory 获取任务执行历史
func (s *taskExecutorService) GetExecutionHistory(ctx context.Context, taskID uint) ([]*ExecutionRecord, error) {
	executions, err := s.taskRepo.GetTaskExecutionHistory(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("获取执行历史失败: %w", err)
	}

	var records []*ExecutionRecord
	for _, exec := range executions {
		record := &ExecutionRecord{
			ID:           exec.ID,
			TaskID:       exec.TaskID,
			SessionID:    exec.ExecutionID,
			BoxID:        exec.BoxID,
			Status:       ExecutionResultStatus(exec.Status),
			StartedAt:    exec.StartedAt,
			CompletedAt:  exec.CompletedAt,
			ErrorMessage: exec.ErrorMessage,
		}

		if exec.CompletedAt != nil {
			duration := exec.CompletedAt.Sub(exec.StartedAt)
			record.Duration = duration
		}

		records = append(records, record)
	}

	return records, nil
}

// StartExecutor 启动执行器
func (s *taskExecutorService) StartExecutor(ctx context.Context) error {
	if s.isRunning {
		return fmt.Errorf("执行器已经在运行")
	}

	s.executorCtx, s.executorCancel = context.WithCancel(ctx)
	s.isRunning = true

	log.Println("任务执行器已启动")
	return nil
}

// StopExecutor 停止执行器
func (s *taskExecutorService) StopExecutor() error {
	if !s.isRunning {
		return fmt.Errorf("执行器未运行")
	}

	if s.executorCancel != nil {
		s.executorCancel()
	}
	s.isRunning = false

	// 停止所有活跃会话
	s.sessionMutex.Lock()
	for sessionID := range s.sessions {
		log.Printf("停止会话: %s", sessionID)
		// 这里应该优雅地停止会话
	}
	s.sessions = make(map[string]*ExecutionSession)
	s.sessionMutex.Unlock()

	log.Println("任务执行器已停止")
	return nil
}

// GetExecutorStatus 获取执行器状态
func (s *taskExecutorService) GetExecutorStatus() *ExecutorStatus {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	// 计算平均执行时间
	var avgExecutionTime time.Duration
	if len(s.executionTimes) > 0 {
		var total time.Duration
		for _, duration := range s.executionTimes {
			total += duration
		}
		avgExecutionTime = total / time.Duration(len(s.executionTimes))
	}

	var lastExecutionAt *time.Time
	if s.totalExecutions > 0 {
		// 这里应该记录最后执行时间，简化实现
		now := time.Now()
		lastExecutionAt = &now
	}

	// 复制当前会话
	sessions := make(map[string]*ExecutionSession)
	for id, session := range s.sessions {
		sessions[id] = session
	}

	return &ExecutorStatus{
		IsRunning:            s.isRunning,
		ActiveSessions:       len(s.sessions),
		TotalExecutions:      s.totalExecutions,
		SuccessfulExecutions: s.successfulExecutions,
		FailedExecutions:     s.failedExecutions,
		AvgExecutionTime:     avgExecutionTime,
		LastExecutionAt:      lastExecutionAt,
		WorkerCount:          1, // 简化实现，固定为1
		QueueLength:          0, // 简化实现，没有队列
		Sessions:             sessions,
	}
}

// 辅助方法

// addLog 添加执行日志
func (s *taskExecutorService) addLog(result *ExecutionResult, level LogLevel, phase ExecutionPhase, message string) {
	logEntry := ExecutionLog{
		Timestamp: time.Now(),
		Level:     level,
		Phase:     phase,
		Message:   message,
	}
	result.Logs = append(result.Logs, logEntry)

	// 同时输出到系统日志
	logMsg := fmt.Sprintf("[Task %d] [%s] [%s] %s", result.TaskID, phase, level, message)
	switch level {
	case LogLevelError:
		log.Printf("ERROR: %s", logMsg)
	case LogLevelWarn:
		log.Printf("WARN: %s", logMsg)
	case LogLevelInfo:
		log.Printf("INFO: %s", logMsg)
	default:
		log.Printf("DEBUG: %s", logMsg)
	}
}

// failExecution 标记执行失败
func (s *taskExecutorService) failExecution(result *ExecutionResult, phase ExecutionPhase, errorMessage string) {
	result.Status = ExecutionResultStatusFailed
	result.ErrorMessage = errorMessage
	completedAt := time.Now()
	result.CompletedAt = &completedAt
	result.Duration = completedAt.Sub(result.StartedAt)

	s.failedExecutions++
	s.executionTimes = append(s.executionTimes, result.Duration)

	s.addLog(result, LogLevelError, phase, errorMessage)
}

// completeExecution 完成执行
func (s *taskExecutorService) completeExecution(result *ExecutionResult) {
	result.Status = ExecutionResultStatusSuccess
	completedAt := time.Now()
	result.CompletedAt = &completedAt
	result.Duration = completedAt.Sub(result.StartedAt)

	s.successfulExecutions++
	s.executionTimes = append(s.executionTimes, result.Duration)

	s.addLog(result, LogLevelInfo, ExecutionPhaseCompleted, "任务执行完成")
}

// monitorSession 监控会话
func (s *taskExecutorService) monitorSession(session *ExecutionSession, result *ExecutionResult) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 更新会话心跳
			session.LastHeartbeat = time.Now()

			// 获取任务状态
			taskStatus, err := s.deploymentService.GetTaskStatusFromBox(context.Background(), session.TaskID, session.BoxID)
			if err != nil {
				s.addLog(result, LogLevelWarn, ExecutionPhaseMonitoring, fmt.Sprintf("获取任务状态失败: %v", err))
				continue
			}

			// 更新进度
			if taskStatus != nil {
				session.Progress = taskStatus.Progress

				// 检查任务是否完成
				if taskStatus.Status == "completed" {
					session.Status = ExecutionSessionStatusCompleted
					session.Phase = ExecutionPhaseCompleted
					return
				}

				if taskStatus.Status == "failed" {
					session.Status = ExecutionSessionStatusFailed
					result.ErrorMessage = taskStatus.Error
					return
				}
			}

		default:
			// 检查会话状态
			if session.Status == ExecutionSessionStatusCompleted ||
				session.Status == ExecutionSessionStatusFailed ||
				session.Status == ExecutionSessionStatusCancelled {
				return
			}

			time.Sleep(1 * time.Second)
		}
	}
}

// waitForCompletion 等待任务完成
func (s *taskExecutorService) waitForCompletion(ctx context.Context, session *ExecutionSession, result *ExecutionResult) error {
	timeout := time.NewTimer(30 * time.Minute) // 30分钟超时
	defer timeout.Stop()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeout.C:
			result.Status = ExecutionResultStatusTimeout
			return fmt.Errorf("任务执行超时")

		case <-ticker.C:
			s.sessionMutex.RLock()
			status := session.Status
			s.sessionMutex.RUnlock()

			switch status {
			case ExecutionSessionStatusCompleted:
				return nil
			case ExecutionSessionStatusFailed:
				return fmt.Errorf("任务执行失败: %s", result.ErrorMessage)
			case ExecutionSessionStatusCancelled:
				result.Status = ExecutionResultStatusCancelled
				return fmt.Errorf("任务被取消")
			}
		}
	}
}
