/*
 * @module api/controllers/monitoring_controller
 * @description 监控管理控制器，提供系统监控、告警、日志等功能
 * @architecture MVC架构 - 控制器层
 * @documentReference DESIGN-000.md
 * @stateFlow HTTP请求处理 -> 业务逻辑处理 -> 数据库操作 -> 响应返回
 * @rules 遵循PostgREST RBAC权限验证，所有操作需要相应权限
 * @dependencies box-manage-service/service
 * @refs DESIGN-000.md
 */

package controllers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"box-manage-service/models"
	"box-manage-service/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// MonitoringController 监控管理控制器
type MonitoringController struct {
	boxMonitoringService   *service.BoxMonitoringService
	conversionService      service.ConversionService
	recordTaskService      service.RecordTaskService
	videoSourceService     service.VideoSourceService
	taskExecutorService    service.TaskExecutorService
	modelDependencyService service.ModelDependencyService
	sseService             service.SSEService
	systemLogService       service.SystemLogService
}

// NewMonitoringController 创建监控控制器实例
func NewMonitoringController(
	boxMonitoringService *service.BoxMonitoringService,
	conversionService service.ConversionService,
	recordTaskService service.RecordTaskService,
	videoSourceService service.VideoSourceService,
	taskExecutorService service.TaskExecutorService,
	modelDependencyService service.ModelDependencyService,
	sseService service.SSEService,
	systemLogService service.SystemLogService,
) *MonitoringController {
	return &MonitoringController{
		boxMonitoringService:   boxMonitoringService,
		conversionService:      conversionService,
		recordTaskService:      recordTaskService,
		videoSourceService:     videoSourceService,
		taskExecutorService:    taskExecutorService,
		modelDependencyService: modelDependencyService,
		sseService:             sseService,
		systemLogService:       systemLogService,
	}
}

// SystemOverviewResponse 系统概览响应
// @Description 系统概览响应数据结构
type SystemOverviewResponse struct {
	Timestamp string            `json:"timestamp"`
	Boxes     BoxesOverview     `json:"boxes"`
	Tasks     TasksOverview     `json:"tasks"`
	Models    ModelsOverview    `json:"models"`
	Videos    VideosOverview    `json:"videos"`
	System    SystemPerformance `json:"system"`
	Services  ServicesStatus    `json:"services"`
}

// BoxesOverview 盒子概览
// @Description 盒子状态概览数据
type BoxesOverview struct {
	Total        int64                   `json:"total"`
	Online       int64                   `json:"online"`
	Offline      int64                   `json:"offline"`
	CpuUsage     float64                 `json:"cpu_usage"`    // 平均 CPU 使用率
	TpuUsage     float64                 `json:"tpu_usage"`    // 平均 TPU 使用率
	Distribution map[string]int          `json:"distribution"` // 按硬件类型分布
	Resources    BoxResourcesAggregation `json:"resources"`
}

// TasksOverview 任务概览
// @Description 任务执行状态概览数据
type TasksOverview struct {
	Total       int64                   `json:"total"`
	Running     int64                   `json:"running"`
	Completed   int64                   `json:"completed"`
	Failed      int64                   `json:"failed"`
	Pending     int64                   `json:"pending"`
	Performance TaskPerformanceOverview `json:"performance"`
}

// ModelsOverview 模型概览
// @Description 模型管理概览数据
type ModelsOverview struct {
	Original        int64                   `json:"original"`
	Converted       int64                   `json:"converted"`
	ConversionTasks ConversionTasksOverview `json:"conversion_tasks"`
	Deployments     int64                   `json:"deployments"`
	StorageUsage    int64                   `json:"storage_usage_mb"`
}

// VideosOverview 视频概览
// @Description 视频处理概览数据
type VideosOverview struct {
	Sources      int64                `json:"sources"`
	Files        int64                `json:"files"`
	RecordTasks  RecordTasksOverview  `json:"record_tasks"`
	ExtractTasks ExtractTasksOverview `json:"extract_tasks"`
	Monitoring   bool                 `json:"monitoring"`
}

// SystemPerformance 系统性能
// @Description 系统性能指标数据
type SystemPerformance struct {
	MemoryUsageMB       int     `json:"memory_usage_mb"`
	ActiveGoroutines    int     `json:"active_goroutines"`
	AverageResponseTime int64   `json:"average_response_time_ms"`
	TotalPolls          int64   `json:"total_polls"`
	SuccessfulPolls     int64   `json:"successful_polls"`
	FailedPolls         int64   `json:"failed_polls"`
	CPUUsage            float64 `json:"cpu_usage_percent"`
}

// ServicesStatus 服务状态
// @Description 各个服务的运行状态
type ServicesStatus struct {
	BoxMonitoring   bool `json:"box_monitoring"`
	TaskExecutor    bool `json:"task_executor"`
	VideoMonitoring bool `json:"video_monitoring"`
	SSEConnections  int  `json:"sse_connections"`
}

// BoxResourcesAggregation 盒子资源聚合
// @Description 盒子资源使用情况聚合数据
type BoxResourcesAggregation struct {
	AvgCPUUsage    float64 `json:"avg_cpu_usage"`    // 平均 CPU 使用率
	AvgTPUUsage    float64 `json:"avg_tpu_usage"`    // 平均 TPU 使用率
	AvgMemoryUsage float64 `json:"avg_memory_usage"` // 平均内存使用率
	AvgTemperature float64 `json:"avg_temperature"`  // 平均温度
	TotalMemoryGB  float64 `json:"total_memory_gb"`  // 总内存(GB)
}

// TaskPerformanceOverview 任务性能概览
// @Description 任务执行性能概览数据
type TaskPerformanceOverview struct {
	AvgFPS               float64 `json:"avg_fps"`
	AvgLatency           float64 `json:"avg_latency_ms"`
	TotalProcessedFrames int64   `json:"total_processed_frames"`
	TotalInferenceCount  int64   `json:"total_inference_count"`
	SuccessRate          float64 `json:"success_rate"`
}

// ConversionTasksOverview 转换任务概览
// @Description 模型转换任务概览数据
type ConversionTasksOverview struct {
	Total       int64   `json:"total"`
	Running     int64   `json:"running"`
	Completed   int64   `json:"completed"`
	Failed      int64   `json:"failed"`
	SuccessRate float64 `json:"success_rate"`
}

// RecordTasksOverview 录制任务概览
// @Description 视频录制任务概览数据
type RecordTasksOverview struct {
	Total     int `json:"total"`
	Recording int `json:"recording"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// ExtractTasksOverview 抽帧任务概览
// @Description 视频抽帧任务概览数据
type ExtractTasksOverview struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// MonitoringMetricsResponse 系统监控指标响应
// @Description 系统监控的详细指标数据
type MonitoringMetricsResponse struct {
	TotalPolls          int64  `json:"total_polls"`
	SuccessfulPolls     int64  `json:"successful_polls"`
	FailedPolls         int64  `json:"failed_polls"`
	AverageResponseTime int64  `json:"average_response_time_ms"`
	ActiveGoroutines    int    `json:"active_goroutines"`
	MemoryUsageMB       int    `json:"memory_usage_mb"`
	OnlineBoxes         int64  `json:"online_boxes"`
	OfflineBoxes        int64  `json:"offline_boxes"`
	TotalBoxes          int64  `json:"total_boxes"`
	MonitoredTasks      int64  `json:"monitored_tasks"`
	RunningTasks        int64  `json:"running_tasks"`
	FailedTasks         int64  `json:"failed_tasks"`
	LastUpdateTime      string `json:"last_update_time"`
}

// PerformanceMetricsResponse 性能指标响应
// @Description 系统性能相关的指标数据
type PerformanceMetricsResponse struct {
	System    SystemPerformanceData `json:"system"`
	Executor  ExecutorPerformance   `json:"executor"`
	Timestamp string                `json:"timestamp"`
}

// SystemPerformanceData 系统性能数据
// @Description 系统性能数据
type SystemPerformanceData struct {
	MemoryUsageMB       int   `json:"memory_usage_mb"`
	ActiveGoroutines    int   `json:"active_goroutines"`
	AverageResponseTime int64 `json:"average_response_time"`
	TotalPolls          int64 `json:"total_polls"`
	SuccessfulPolls     int64 `json:"successful_polls"`
	FailedPolls         int64 `json:"failed_polls"`
}

// ExecutorPerformance 执行器性能数据
// @Description 任务执行器性能数据
type ExecutorPerformance struct {
	ActiveSessions       int    `json:"active_sessions"`
	TotalExecutions      int64  `json:"total_executions"`
	SuccessfulExecutions int64  `json:"successful_executions"`
	FailedExecutions     int64  `json:"failed_executions"`
	AvgExecutionTime     string `json:"avg_execution_time"`
	WorkerCount          int    `json:"worker_count"`
	QueueLength          int    `json:"queue_length"`
}

// TaskMetricsResponse 任务指标响应
// @Description 任务相关的统计指标
type TaskMetricsResponse struct {
	ConversionTasks interface{} `json:"conversion_tasks"`
	RecordTasks     interface{} `json:"record_tasks"`
	Timestamp       string      `json:"timestamp"`
}

// ResourceUsageResponse 资源使用情况响应
// @Description 系统资源使用情况统计
type ResourceUsageResponse struct {
	MemoryUsageMB       int               `json:"memory_usage_mb"`
	ActiveGoroutines    int               `json:"active_goroutines"`
	AverageResponseTime int64             `json:"average_response_time"`
	Boxes               BoxResourceUsage  `json:"boxes"`
	Tasks               TaskResourceUsage `json:"tasks"`
	Timestamp           string            `json:"timestamp"`
}

// BoxResourceUsage 盒子资源使用
// @Description 盒子资源使用统计
type BoxResourceUsage struct {
	Total   int64 `json:"total"`
	Online  int64 `json:"online"`
	Offline int64 `json:"offline"`
}

// TaskResourceUsage 任务资源使用
// @Description 任务资源使用统计
type TaskResourceUsage struct {
	Monitored int64 `json:"monitored"`
	Running   int64 `json:"running"`
	Failed    int64 `json:"failed"`
}

// HealthCheckResponse 健康检查响应
// @Description 系统健康检查结果
type HealthCheckResponse struct {
	Status    string          `json:"status"`
	Timestamp string          `json:"timestamp"`
	Services  map[string]bool `json:"services"`
	Uptime    string          `json:"uptime"`
}

// LogStatisticsResponse 日志统计响应
// @Description 日志统计信息响应
type LogStatisticsResponse struct {
	Statistics interface{}   `json:"statistics"`
	TimeRange  TimeRangeData `json:"time_range"`
}

// TimeRangeData 时间范围数据
// @Description 时间范围数据
type TimeRangeData struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// GetSystemOverview 获取系统概览
// @Summary 获取系统概览
// @Description 获取整个系统的概览信息，包括盒子状态、任务统计、模型信息、视频处理、系统性能和服务状态等
// @Tags 监控管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=SystemOverviewResponse} "获取系统概览成功"
// @Failure 500 {object} ErrorResponse "服务未初始化或内部错误"
// @Router /monitoring/overview [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetSystemOverview(w http.ResponseWriter, r *http.Request) {
	// 检查服务是否已初始化
	if c.boxMonitoringService == nil || c.conversionService == nil || c.recordTaskService == nil {
		render.Render(w, r, InternalErrorResponse("监控服务未初始化", nil))
		return
	}

	ctx := context.Background()

	// 获取盒子监控概览
	boxOverview, err := c.boxMonitoringService.GetSystemOverview()
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取盒子概览失败", err))
		return
	}

	// 获取转换任务统计
	conversionStats, err := c.conversionService.GetConversionStatistics(ctx, nil)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取转换统计失败", err))
		return
	}

	// 获取录制任务统计
	recordStats, err := c.recordTaskService.GetTaskStatistics(ctx, nil)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取录制统计失败", err))
		return
	}

	// 获取执行器状态
	executorStatus := c.taskExecutorService.GetExecutorStatus()

	// 获取SSE连接统计
	sseStats := c.sseService.GetConnectionStats()

	// 获取视频监控状态
	videoMonitoringStatus := c.videoSourceService.GetMonitoringStatus()

	// 提取资源聚合数据
	resourcesData := boxOverview["resources"].(map[string]interface{})
	avgCPUUsage := resourcesData["avg_cpu_usage"].(float64)
	avgTPUUsage := resourcesData["avg_tpu_usage"].(float64)
	avgMemoryUsage := resourcesData["avg_memory_usage"].(float64)
	avgTemperature := resourcesData["avg_temperature"].(float64)
	totalMemoryGB := resourcesData["total_memory_gb"].(float64)

	response := &SystemOverviewResponse{
		Timestamp: time.Now().Format(time.RFC3339),
		Boxes: BoxesOverview{
			Total:    boxOverview["boxes"].(map[string]interface{})["total"].(int64),
			Online:   boxOverview["boxes"].(map[string]interface{})["online"].(int64),
			Offline:  boxOverview["boxes"].(map[string]interface{})["offline"].(int64),
			CpuUsage: avgCPUUsage,
			TpuUsage: avgTPUUsage,
			Resources: BoxResourcesAggregation{
				AvgCPUUsage:    avgCPUUsage,
				AvgTPUUsage:    avgTPUUsage,
				AvgMemoryUsage: avgMemoryUsage,
				AvgTemperature: avgTemperature,
				TotalMemoryGB:  totalMemoryGB,
			},
		},
		Tasks: TasksOverview{
			Running: boxOverview["tasks"].(map[string]interface{})["running"].(int64),
			Failed:  boxOverview["tasks"].(map[string]interface{})["failed"].(int64),
			Performance: TaskPerformanceOverview{
				SuccessRate: calculateTaskSuccessRate(boxOverview["tasks"].(map[string]interface{})),
			},
		},
		Models: ModelsOverview{
			ConversionTasks: ConversionTasksOverview{
				Total:       conversionStats.TotalTasks,
				Running:     conversionStats.RunningTasks,
				Completed:   conversionStats.CompletedTasks,
				Failed:      conversionStats.FailedTasks,
				SuccessRate: conversionStats.SuccessRate,
			},
		},
		Videos: VideosOverview{
			RecordTasks: RecordTasksOverview{
				Total:     recordStats.TotalCount,
				Recording: recordStats.RecordingCount,
				Completed: recordStats.CompletedCount,
				Failed:    recordStats.FailedCount,
			},
			Monitoring: videoMonitoringStatus["is_monitoring"].(bool),
		},
		System: SystemPerformance{
			MemoryUsageMB:       boxOverview["performance"].(map[string]interface{})["memory_usage_mb"].(int),
			ActiveGoroutines:    boxOverview["performance"].(map[string]interface{})["active_goroutines"].(int),
			AverageResponseTime: boxOverview["performance"].(map[string]interface{})["average_response_time"].(int64),
			TotalPolls:          boxOverview["performance"].(map[string]interface{})["total_polls"].(int64),
			SuccessfulPolls:     boxOverview["performance"].(map[string]interface{})["successful_polls"].(int64),
			FailedPolls:         boxOverview["performance"].(map[string]interface{})["failed_polls"].(int64),
		},
		Services: ServicesStatus{
			BoxMonitoring:   c.boxMonitoringService.IsRunning(),
			TaskExecutor:    executorStatus.IsRunning,
			VideoMonitoring: videoMonitoringStatus["is_monitoring"].(bool),
			SSEConnections:  sseStats.TotalConnections,
		},
	}

	render.Render(w, r, SuccessResponse("获取系统概览成功", response))
}

// GetSystemMetrics 获取系统指标
// @Summary 获取系统指标
// @Description 获取系统监控的详细指标数据，包括盒子状态、任务统计、资源使用等监控数据
// @Tags 监控管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=MonitoringMetricsResponse} "获取系统指标成功"
// @Failure 500 {object} ErrorResponse "监控服务未初始化"
// @Router /monitoring/metrics [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetSystemMetrics(w http.ResponseWriter, r *http.Request) {
	if c.boxMonitoringService == nil {
		render.Render(w, r, InternalErrorResponse("监控服务未初始化", nil))
		return
	}

	// 获取监控指标
	metrics := c.boxMonitoringService.GetMetrics()

	render.Render(w, r, SuccessResponse("获取系统指标成功", metrics))
}

// GetPerformanceMetrics 获取性能指标
// @Summary 获取性能指标
// @Description 获取系统性能相关的指标数据，包括执行器状态、系统资源使用情况等
// @Tags 监控管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=PerformanceMetricsResponse} "获取性能指标成功"
// @Failure 500 {object} ErrorResponse "获取系统概览失败"
// @Router /monitoring/performance [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetPerformanceMetrics(w http.ResponseWriter, r *http.Request) {
	// 获取执行器状态
	executorStatus := c.taskExecutorService.GetExecutorStatus()

	// 获取系统概览
	systemOverview, err := c.boxMonitoringService.GetSystemOverview()
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取系统概览失败", err))
		return
	}

	performanceData := map[string]interface{}{
		"system": systemOverview["performance"],
		"executor": map[string]interface{}{
			"active_sessions":       executorStatus.ActiveSessions,
			"total_executions":      executorStatus.TotalExecutions,
			"successful_executions": executorStatus.SuccessfulExecutions,
			"failed_executions":     executorStatus.FailedExecutions,
			"avg_execution_time":    executorStatus.AvgExecutionTime.String(),
			"worker_count":          executorStatus.WorkerCount,
			"queue_length":          executorStatus.QueueLength,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	render.Render(w, r, SuccessResponse("获取性能指标成功", performanceData))
}

// GetBoxMetrics 获取盒子指标
// @Summary 获取指定盒子的监控指标
// @Description 获取指定盒子的详细监控指标，包括硬件状态、资源使用等信息
// @Tags 监控管理
// @Accept json
// @Produce json
// @Param id path string true "盒子ID"
// @Success 200 {object} APIResponse "获取盒子指标成功"
// @Failure 400 {object} ErrorResponse "缺少盒子ID参数或无效的盒子ID"
// @Failure 500 {object} ErrorResponse "刷新盒子状态失败"
// @Router /monitoring/boxes/{id}/metrics [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetBoxMetrics(w http.ResponseWriter, r *http.Request) {
	boxIDStr := chi.URLParam(r, "id")
	if boxIDStr == "" {
		render.Render(w, r, BadRequestResponse("缺少盒子ID参数", nil))
		return
	}

	boxID, err := strconv.ParseUint(boxIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	// 刷新盒子状态
	err = c.boxMonitoringService.RefreshBoxStatus(uint(boxID))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("刷新盒子状态失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取盒子指标成功", nil))
}

// GetTaskMetrics 获取任务指标
// @Summary 获取任务相关的监控指标
// @Description 获取任务相关的统计指标，包括转换任务和录制任务的统计信息
// @Tags 监控管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=TaskMetricsResponse} "获取任务指标成功"
// @Failure 500 {object} ErrorResponse "获取转换统计失败或获取录制统计失败"
// @Router /monitoring/tasks/metrics [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetTaskMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// 获取转换统计
	conversionStats, err := c.conversionService.GetConversionStatistics(ctx, nil)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取转换统计失败", err))
		return
	}

	// 获取录制统计
	recordStats, err := c.recordTaskService.GetTaskStatistics(ctx, nil)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取录制统计失败", err))
		return
	}

	taskMetrics := map[string]interface{}{
		"conversion_tasks": conversionStats,
		"record_tasks":     recordStats,
		"timestamp":        time.Now().Format(time.RFC3339),
	}

	render.Render(w, r, SuccessResponse("获取任务指标成功", taskMetrics))
}

// GetResourceUsage 获取资源使用情况
// @Summary 获取系统资源使用情况
// @Description 获取系统资源使用情况统计，包括内存使用、协程数量、响应时间等系统资源指标
// @Tags 监控管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=ResourceUsageResponse} "获取资源使用情况成功"
// @Router /monitoring/resource-usage [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetResourceUsage(w http.ResponseWriter, r *http.Request) {
	// 获取监控指标
	metrics := c.boxMonitoringService.GetMetrics()

	resourceUsage := map[string]interface{}{
		"memory_usage_mb":       metrics.MemoryUsageMB,
		"active_goroutines":     metrics.ActiveGoroutines,
		"average_response_time": metrics.AverageResponseTime,
		"boxes": map[string]interface{}{
			"total":   metrics.TotalBoxes,
			"online":  metrics.OnlineBoxes,
			"offline": metrics.OfflineBoxes,
		},
		"tasks": map[string]interface{}{
			"monitored": metrics.MonitoredTasks,
			"running":   metrics.RunningTasks,
			"failed":    metrics.FailedTasks,
		},
		"timestamp": metrics.LastUpdateTime.Format(time.RFC3339),
	}

	render.Render(w, r, SuccessResponse("获取资源使用情况成功", resourceUsage))
}

// GetSystemLogs 获取系统日志
// @Summary 获取系统日志
// @Description 获取系统运行日志，支持按级别、来源、时间等条件过滤
// @Tags 监控管理
// @Accept json
// @Produce json
// @Param level query string false "日志级别过滤 (debug, info, warn, error, fatal)"
// @Param source query string false "日志来源过滤 (box_monitoring_service, task_executor_service, etc.)"
// @Param source_id query string false "来源ID过滤"
// @Param user_id query int false "用户ID过滤"
// @Param request_id query string false "请求ID过滤"
// @Param start_time query string false "开始时间 (RFC3339格式)"
// @Param end_time query string false "结束时间 (RFC3339格式)"
// @Param keyword query string false "关键词搜索"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(100)
// @Success 200 {object} APIResponse{data=service.GetLogsResponse} "获取系统日志成功"
// @Failure 500 {object} ErrorResponse "获取系统日志失败"
// @Router /monitoring/logs [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetSystemLogs(w http.ResponseWriter, r *http.Request) {
	if c.systemLogService == nil {
		render.Render(w, r, InternalErrorResponse("日志服务未初始化", nil))
		return
	}

	// 解析查询参数
	req := &service.GetLogsRequest{
		Source:    r.URL.Query().Get("source"),
		SourceID:  r.URL.Query().Get("source_id"),
		RequestID: r.URL.Query().Get("request_id"),
		Keyword:   r.URL.Query().Get("keyword"),
		Page:      1,
		PageSize:  100,
	}

	// 解析日志级别
	if levelStr := r.URL.Query().Get("level"); levelStr != "" {
		level := models.LogLevel(levelStr)
		req.Level = &level
	}

	// 解析用户ID
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uid := uint(userID)
			req.UserID = &uid
		}
	}

	// 解析分页参数
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			req.Page = page
		}
	}
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 {
			req.PageSize = pageSize
		}
	}

	// 解析时间参数
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			req.StartTime = &startTime
		}
	}
	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			req.EndTime = &endTime
		}
	}

	// 获取日志
	ctx := context.Background()
	response, err := c.systemLogService.GetLogs(ctx, req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取系统日志失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取系统日志成功", response))
}

// GetLogStatistics 获取日志统计
// @Summary 获取日志统计信息
// @Description 获取指定时间范围内的日志统计信息，包括按级别、来源、时间的统计
// @Tags 监控管理
// @Accept json
// @Produce json
// @Param start_time query string false "开始时间 (RFC3339格式)" default("24小时前")
// @Param end_time query string false "结束时间 (RFC3339格式)" default("当前时间")
// @Success 200 {object} APIResponse{data=LogStatisticsResponse} "获取日志统计成功"
// @Failure 500 {object} ErrorResponse "获取日志统计失败"
// @Router /monitoring/logs/statistics [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetLogStatistics(w http.ResponseWriter, r *http.Request) {
	if c.systemLogService == nil {
		render.Render(w, r, InternalErrorResponse("日志服务未初始化", nil))
		return
	}

	// 默认查询最近24小时的日志
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	// 解析时间参数
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = t
		}
	}
	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = t
		}
	}

	// 获取统计信息
	ctx := context.Background()
	statistics, err := c.systemLogService.GetLogStatistics(ctx, startTime, endTime)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取日志统计失败", err))
		return
	}

	// 添加时间范围信息
	result := map[string]interface{}{
		"statistics": statistics,
		"time_range": map[string]interface{}{
			"start_time": startTime.Format(time.RFC3339),
			"end_time":   endTime.Format(time.RFC3339),
		},
	}

	render.Render(w, r, SuccessResponse("获取日志统计成功", result))
}

// HealthCheck 健康检查
// @Summary 系统健康检查
// @Description 检查系统各个服务的健康状态，包括监控服务、任务执行器、视频监控等
// @Tags 监控管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=HealthCheckResponse} "健康检查成功"
// @Router /monitoring/health-check [get]
// @Security ApiKeyAuth
func (c *MonitoringController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"services": map[string]bool{
			"box_monitoring":   c.boxMonitoringService.IsRunning(),
			"task_executor":    c.taskExecutorService.GetExecutorStatus().IsRunning,
			"video_monitoring": c.videoSourceService.GetMonitoringStatus()["is_monitoring"].(bool),
		},
		"uptime": time.Since(time.Now().Add(-24 * time.Hour)).String(), // TODO: 实际启动时间
	}

	render.Render(w, r, SuccessResponse("健康检查成功", health))
}

// calculateTaskSuccessRate 计算任务成功率
func calculateTaskSuccessRate(tasks map[string]interface{}) float64 {
	running, _ := tasks["running"].(int64)
	failed, _ := tasks["failed"].(int64)

	total := running + failed
	if total == 0 {
		return 100.0
	}

	return float64(running) / float64(total) * 100.0
}

// TopNMetricsRequest TopN指标请求参数
// @Description TopN指标查询请求参数
type TopNMetricsRequest struct {
	MetricName string `json:"metric_name"` // 指标名称：cpu_usage, tpu_usage, memory_usage, temperature, load 等
	TopN       int    `json:"top_n"`       // 返回数量，默认10
	Order      string `json:"order"`       // 排序方式：desc(降序), asc(升序)，默认desc
}

// TopNMetricsItem TopN指标项
// @Description TopN指标查询结果项
type TopNMetricsItem struct {
	BoxID       uint    `json:"box_id"`       // 盒子ID
	BoxName     string  `json:"box_name"`     // 盒子名称
	IPAddress   string  `json:"ip_address"`   // IP地址
	Status      string  `json:"status"`       // 状态
	MetricName  string  `json:"metric_name"`  // 指标名称
	MetricValue float64 `json:"metric_value"` // 指标值
	Unit        string  `json:"unit"`         // 单位
}

// TopNMetricsResponse TopN指标响应
// @Description TopN指标查询结果
type TopNMetricsResponse struct {
	MetricName string            `json:"metric_name"` // 指标名称
	TopN       int               `json:"top_n"`       // 返回数量
	Order      string            `json:"order"`       // 排序方式
	Items      []TopNMetricsItem `json:"items"`       // 指标项列表
	Timestamp  string            `json:"timestamp"`   // 查询时间
}

// AllMetricsTopNResponse 所有指标TopN响应
// @Description 所有主要指标的TopN排名结果
type AllMetricsTopNResponse struct {
	CPUUsage    *TopNMetricsResponse `json:"cpu_usage,omitempty"`
	TPUUsage    *TopNMetricsResponse `json:"tpu_usage,omitempty"`
	MemoryUsage *TopNMetricsResponse `json:"memory_usage,omitempty"`
	Temperature *TopNMetricsResponse `json:"temperature,omitempty"`
	Load        *TopNMetricsResponse `json:"load,omitempty"`
	Timestamp   string               `json:"timestamp"`
}

// GetTopNBoxesByMetric 获取指标TopN排序的盒子列表
// @Summary 获取指标TopN排序的盒子列表
// @Description 根据指定的监控指标获取排名前N的盒子列表，支持CPU使用率、TPU使用率、内存使用率、温度、负载等指标
// @Tags 监控管理
// @Accept json
// @Produce json
// @Param metric query string true "指标名称" Enums(cpu_usage, tpu_usage, memory_usage, temperature, load, load5, load15, disk_usage, npu_memory, swap_usage, io_read, io_write, net_recv, net_sent, procs)
// @Param n query int false "返回数量，默认10" default(10)
// @Param order query string false "排序方式：desc(降序), asc(升序)" default(desc) Enums(desc, asc)
// @Success 200 {object} APIResponse{data=TopNMetricsResponse} "获取TopN指标成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "服务未初始化或内部错误"
// @Router /monitoring/topn [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetTopNBoxesByMetric(w http.ResponseWriter, r *http.Request) {
	if c.boxMonitoringService == nil {
		render.Render(w, r, InternalErrorResponse("监控服务未初始化", nil))
		return
	}

	// 解析参数
	metricName := r.URL.Query().Get("metric")
	if metricName == "" {
		metricName = "cpu_usage" // 默认CPU使用率
	}

	// 验证指标名称
	validMetrics := map[string]bool{
		"cpu_usage":    true,
		"tpu_usage":    true,
		"memory_usage": true,
		"temperature":  true,
		"load":         true,
		"load5":        true,
		"load15":       true,
		"disk_usage":   true,
		"npu_memory":   true,
		"swap_usage":   true,
		"io_read":      true,
		"io_write":     true,
		"net_recv":     true,
		"net_sent":     true,
		"procs":        true,
	}

	if !validMetrics[metricName] {
		render.Render(w, r, BadRequestResponse("无效的指标名称，支持: cpu_usage, tpu_usage, memory_usage, temperature, load, load5, load15, disk_usage, npu_memory, swap_usage, io_read, io_write, net_recv, net_sent, procs", nil))
		return
	}

	// 解析TopN数量
	topN := 10
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if n, err := strconv.Atoi(nStr); err == nil && n > 0 {
			topN = n
		}
	}

	// 解析排序方式
	descOrder := true
	if order := r.URL.Query().Get("order"); order == "asc" {
		descOrder = false
	}

	// 获取TopN数据
	result, err := c.boxMonitoringService.GetTopNBoxesByMetric(metricName, topN, descOrder)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取TopN指标失败", err))
		return
	}

	// 转换为响应格式
	response := &TopNMetricsResponse{
		MetricName: result.MetricName,
		TopN:       result.TopN,
		Order:      result.Order,
		Timestamp:  result.Timestamp,
		Items:      make([]TopNMetricsItem, len(result.Items)),
	}

	for i, item := range result.Items {
		response.Items[i] = TopNMetricsItem{
			BoxID:       item.BoxID,
			BoxName:     item.BoxName,
			IPAddress:   item.IPAddress,
			Status:      item.Status,
			MetricName:  item.MetricName,
			MetricValue: item.MetricValue,
			Unit:        item.Unit,
		}
	}

	render.Render(w, r, SuccessResponse("获取TopN指标成功", response))
}

// GetAllMetricsTopN 获取所有主要指标的TopN排名
// @Summary 获取所有主要指标的TopN排名
// @Description 一次性获取CPU、TPU、内存、温度、负载等主要指标的TopN排名
// @Tags 监控管理
// @Accept json
// @Produce json
// @Param n query int false "返回数量，默认10" default(10)
// @Success 200 {object} APIResponse{data=AllMetricsTopNResponse} "获取所有指标TopN成功"
// @Failure 500 {object} ErrorResponse "服务未初始化或内部错误"
// @Router /monitoring/topn/all [get]
// @Security ApiKeyAuth
func (c *MonitoringController) GetAllMetricsTopN(w http.ResponseWriter, r *http.Request) {
	if c.boxMonitoringService == nil {
		render.Render(w, r, InternalErrorResponse("监控服务未初始化", nil))
		return
	}

	// 解析TopN数量
	topN := 10
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if n, err := strconv.Atoi(nStr); err == nil && n > 0 {
			topN = n
		}
	}

	// 获取所有指标的TopN
	results, err := c.boxMonitoringService.GetAllMetricsTopN(topN)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取所有指标TopN失败", err))
		return
	}

	// 转换为响应格式
	response := &AllMetricsTopNResponse{
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// 转换各个指标
	convertToResponse := func(result *service.TopNMetricsResult) *TopNMetricsResponse {
		if result == nil {
			return nil
		}
		resp := &TopNMetricsResponse{
			MetricName: result.MetricName,
			TopN:       result.TopN,
			Order:      result.Order,
			Timestamp:  result.Timestamp,
			Items:      make([]TopNMetricsItem, len(result.Items)),
		}
		for i, item := range result.Items {
			resp.Items[i] = TopNMetricsItem{
				BoxID:       item.BoxID,
				BoxName:     item.BoxName,
				IPAddress:   item.IPAddress,
				Status:      item.Status,
				MetricName:  item.MetricName,
				MetricValue: item.MetricValue,
				Unit:        item.Unit,
			}
		}
		return resp
	}

	response.CPUUsage = convertToResponse(results["cpu_usage"])
	response.TPUUsage = convertToResponse(results["tpu_usage"])
	response.MemoryUsage = convertToResponse(results["memory_usage"])
	response.Temperature = convertToResponse(results["temperature"])
	response.Load = convertToResponse(results["load"])

	render.Render(w, r, SuccessResponse("获取所有指标TopN成功", response))
}
