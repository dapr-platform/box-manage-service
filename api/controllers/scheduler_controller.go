/*
 * @module api/controllers/scheduler_controller
 * @description 任务调度控制器 - 管理任务的自动调度和手动调度操作
 * @architecture RESTful API Controller
 * @documentReference REQ-005: 任务管理功能
 * @stateFlow HTTP请求 -> 调度服务调用 -> 响应返回
 * @rules 任务调度基于标签亲和性和盒子资源进行匹配
 * @dependencies service.TaskSchedulerService, service.TaskExecutorService
 */

package controllers

import (
	"box-manage-service/repository"
	"box-manage-service/service"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// SchedulerController 任务调度控制器
type SchedulerController struct {
	schedulerService service.TaskSchedulerService
	executorService  service.TaskExecutorService
	historyRepo      repository.TaskScheduleHistoryRepository
}

// NewSchedulerController 创建任务调度控制器实例
func NewSchedulerController(
	schedulerService service.TaskSchedulerService,
	executorService service.TaskExecutorService,
	historyRepo repository.TaskScheduleHistoryRepository,
) *SchedulerController {
	return &SchedulerController{
		schedulerService: schedulerService,
		executorService:  executorService,
		historyRepo:      historyRepo,
	}
}

// ScheduleTaskRequest 调度单个任务请求
type ScheduleTaskRequest struct {
	BoxID *uint `json:"box_id,omitempty"` // 可选：指定目标盒子
}

// ScheduleTaskResponse 调度任务响应
type ScheduleTaskResponse struct {
	TaskID     uint                `json:"task_id"`
	TaskName   string              `json:"task_name"`
	Success    bool                `json:"success"`
	BoxID      *uint               `json:"box_id,omitempty"`
	BoxName    string              `json:"box_name,omitempty"`
	Score      float64             `json:"score,omitempty"`
	Reason     string              `json:"reason"`
	Candidates []*service.BoxScore `json:"candidates,omitempty"`
}

// ScheduleAllResponse 批量调度响应
type ScheduleAllResponse struct {
	TotalTasks     int                     `json:"total_tasks"`
	ScheduledTasks int                     `json:"scheduled_tasks"`
	FailedTasks    int                     `json:"failed_tasks"`
	TaskResults    []*ScheduleTaskResponse `json:"task_results"`
	Summary        map[string]int          `json:"summary"`
}

// SchedulerStatusResponse 调度器状态响应
type SchedulerStatusResponse struct {
	IsRunning            bool   `json:"is_running"`
	ActiveSessions       int    `json:"active_sessions"`
	TotalExecutions      int64  `json:"total_executions"`
	SuccessfulExecutions int64  `json:"successful_executions"`
	FailedExecutions     int64  `json:"failed_executions"`
	AvgExecutionTime     string `json:"avg_execution_time"`
	WorkerCount          int    `json:"worker_count"`
	QueueLength          int    `json:"queue_length"`
}

// BoxScoreResponse 盒子评分响应
type BoxScoreResponse struct {
	BoxID       uint     `json:"box_id"`
	BoxName     string   `json:"box_name"`
	Score       float64  `json:"score"`
	IsOnline    bool     `json:"is_online"`
	HasCapacity bool     `json:"has_capacity"`
	TagMatches  int      `json:"tag_matches"`
	Reasons     []string `json:"reasons"`
}

// ScheduleTask 调度单个任务
// @Summary 调度单个任务到合适的盒子
// @Description 根据任务的亲和性标签和盒子资源，调度任务到最合适的盒子
// @Tags 任务调度
// @Accept json
// @Produce json
// @Param taskId path int true "任务ID"
// @Param request body ScheduleTaskRequest false "调度参数"
// @Success 200 {object} APIResponse{data=ScheduleTaskResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/scheduler/schedule/{taskId} [post]
func (c *SchedulerController) ScheduleTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 获取任务ID
	taskIDStr := chi.URLParam(r, "taskId")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	// 调用调度服务
	result, err := c.schedulerService.ScheduleTask(ctx, uint(taskID))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("调度任务失败", err))
		return
	}

	// 转换为响应格式
	response := &ScheduleTaskResponse{
		TaskID:   result.TaskID,
		TaskName: result.TaskName,
		Success:  result.Success,
		BoxID:    result.BoxID,
		BoxName:  result.BoxName,
		Score:    result.Score,
		Reason:   result.Reason,
	}

	// 添加候选盒子信息
	if len(result.Candidates) > 0 {
		response.Candidates = result.Candidates
	}

	render.Render(w, r, SuccessResponse("任务调度完成", response))
}

// SchedulePendingTasks 调度所有待处理的自动调度任务
// @Summary 调度所有待处理任务
// @Description 自动调度所有启用自动调度且处于待处理状态的任务
// @Tags 任务调度
// @Produce json
// @Success 200 {object} APIResponse{data=ScheduleAllResponse}
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/scheduler/schedule/pending [post]
func (c *SchedulerController) SchedulePendingTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 调用调度服务
	result, err := c.schedulerService.ScheduleAutoTasks(ctx)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("批量调度失败", err))
		return
	}

	// 转换为响应格式
	response := &ScheduleAllResponse{
		TotalTasks:     result.TotalTasks,
		ScheduledTasks: result.ScheduledTasks,
		FailedTasks:    result.FailedTasks,
		Summary:        result.Summary,
		TaskResults:    make([]*ScheduleTaskResponse, 0, len(result.TaskResults)),
	}

	for _, taskResult := range result.TaskResults {
		response.TaskResults = append(response.TaskResults, &ScheduleTaskResponse{
			TaskID:   taskResult.TaskID,
			TaskName: taskResult.TaskName,
			Success:  taskResult.Success,
			BoxID:    taskResult.BoxID,
			BoxName:  taskResult.BoxName,
			Score:    taskResult.Score,
			Reason:   taskResult.Reason,
		})
	}

	render.Render(w, r, SuccessResponse("批量调度完成", response))
}

// GetQueueStatus 获取调度队列状态
// @Summary 获取调度器状态
// @Description 获取任务调度器的当前状态和统计信息
// @Tags 任务调度
// @Produce json
// @Success 200 {object} APIResponse{data=SchedulerStatusResponse}
// @Router /api/v1/scheduler/queue/status [get]
func (c *SchedulerController) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
	status := c.executorService.GetExecutorStatus()

	response := &SchedulerStatusResponse{
		IsRunning:            status.IsRunning,
		ActiveSessions:       status.ActiveSessions,
		TotalExecutions:      status.TotalExecutions,
		SuccessfulExecutions: status.SuccessfulExecutions,
		FailedExecutions:     status.FailedExecutions,
		AvgExecutionTime:     status.AvgExecutionTime.String(),
		WorkerCount:          status.WorkerCount,
		QueueLength:          status.QueueLength,
	}

	render.Render(w, r, SuccessResponse("获取调度器状态成功", response))
}

// FindBestBox 为任务查找最佳盒子
// @Summary 查找最佳盒子
// @Description 根据任务的亲和性标签和资源需求，查找所有兼容的盒子并评分
// @Tags 任务调度
// @Produce json
// @Param taskId path int true "任务ID"
// @Success 200 {object} APIResponse{data=[]BoxScoreResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/scheduler/find-best-box/{taskId} [get]
func (c *SchedulerController) FindBestBox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 获取任务ID
	taskIDStr := chi.URLParam(r, "taskId")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	// 查找兼容盒子
	boxScores, err := c.schedulerService.FindCompatibleBoxes(ctx, uint(taskID))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("查找兼容盒子失败", err))
		return
	}

	// 转换为响应格式
	response := make([]*BoxScoreResponse, 0, len(boxScores))
	for _, score := range boxScores {
		response = append(response, &BoxScoreResponse{
			BoxID:       score.BoxID,
			BoxName:     score.BoxName,
			Score:       score.Score,
			IsOnline:    score.IsOnline,
			HasCapacity: score.HasCapacity,
			TagMatches:  score.TagMatches,
			Reasons:     score.Reasons,
		})
	}

	render.Render(w, r, SuccessResponse("查找兼容盒子成功", response))
}

// StartScheduler 启动调度器
// @Summary 启动调度器
// @Description 启动任务执行器，开始接受任务调度
// @Tags 任务调度
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/scheduler/start [post]
func (c *SchedulerController) StartScheduler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := c.executorService.StartExecutor(ctx); err != nil {
		render.Render(w, r, InternalErrorResponse("启动调度器失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("调度器已启动", nil))
}

// StopScheduler 停止调度器
// @Summary 停止调度器
// @Description 停止任务执行器，暂停任务调度
// @Tags 任务调度
// @Produce json
// @Success 200 {object} APIResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/scheduler/stop [post]
func (c *SchedulerController) StopScheduler(w http.ResponseWriter, r *http.Request) {
	if err := c.executorService.StopExecutor(); err != nil {
		render.Render(w, r, InternalErrorResponse("停止调度器失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("调度器已停止", nil))
}

// GetScheduleHistory 获取任务调度历史
// @Summary 获取任务调度历史
// @Description 分页查询任务调度历史记录，支持按任务、盒子、触发类型、关键词、时间范围组合过滤
// @Tags 任务调度
// @Produce json
// @Param task_id query int false "任务ID"
// @Param box_id query int false "盒子ID"
// @Param trigger_type query string false "触发类型" Enums(manual,auto)
// @Param keyword query string false "关键词（匹配任务名/盒子名）"
// @Param start_time query string false "起始时间（RFC3339 或 2006-01-02）"
// @Param end_time query string false "结束时间（RFC3339 或 2006-01-02）"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} APIResponse
// @Router /api/v1/scheduler/history [get]
func (c *SchedulerController) GetScheduleHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, pageSize := 1, 20
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(q.Get("page_size")); err == nil && ps > 0 && ps <= 100 {
		pageSize = ps
	}

	ctx := r.Context()

	// 组装过滤条件（各条件可自由组合：任务/盒子/触发类型/关键词/时间范围）
	filter := repository.TaskScheduleHistoryFilter{}

	if v := strings.TrimSpace(q.Get("task_id")); v != "" {
		if id, err := strconv.ParseUint(v, 10, 32); err == nil {
			uid := uint(id)
			filter.TaskID = &uid
		}
	}
	if v := strings.TrimSpace(q.Get("box_id")); v != "" {
		if id, err := strconv.ParseUint(v, 10, 32); err == nil {
			uid := uint(id)
			filter.BoxID = &uid
		}
	}
	// 触发类型：兼容 trigger_type / trigger 两个参数名，取值 manual / auto
	trigger := strings.TrimSpace(q.Get("trigger_type"))
	if trigger == "" {
		trigger = strings.TrimSpace(q.Get("trigger"))
	}
	if trigger == "manual" || trigger == "auto" {
		filter.Trigger = trigger
	}
	// 关键词：匹配任务名/盒子名
	filter.Keyword = strings.TrimSpace(q.Get("keyword"))
	// 时间范围：支持 RFC3339、日期、日期时间与时间戳
	if t, ok := parseQueryTime(q.Get("start_time")); ok {
		filter.StartTime = &t
	}
	if t, ok := parseQueryTime(q.Get("end_time")); ok {
		filter.EndTime = &t
	}

	list, total, err := c.historyRepo.FindWithFilter(ctx, filter, page, pageSize)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("查询调度历史失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取调度历史成功", map[string]interface{}{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}))
}

// parseQueryTime 解析查询时间参数，支持 RFC3339、日期(2006-01-02)、日期时间与秒/毫秒时间戳。
func parseQueryTime(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t, true
		}
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		if n > 1e12 {
			return time.UnixMilli(n), true
		}
		return time.Unix(n, 0), true
	}
	return time.Time{}, false
}
