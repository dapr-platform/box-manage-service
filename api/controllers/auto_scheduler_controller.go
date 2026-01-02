/*
 * @module api/controllers/auto_scheduler_controller
 * @description 自动调度控制器
 * @architecture 控制层
 * @documentReference REQ-005: 任务管理功能
 * @stateFlow HTTP请求 -> 参数验证 -> 业务处理 -> 响应返回
 * @rules 自动调度器的启动、停止、状态查询和手动触发
 * @dependencies chi, service
 */

package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"box-manage-service/service"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// AutoSchedulerController 自动调度控制器
type AutoSchedulerController struct {
	autoSchedulerService service.AutoSchedulerService
}

// NewAutoSchedulerController 创建自动调度控制器实例
func NewAutoSchedulerController(autoSchedulerService service.AutoSchedulerService) *AutoSchedulerController {
	return &AutoSchedulerController{
		autoSchedulerService: autoSchedulerService,
	}
}

// TriggerScheduleRequest 手动触发调度请求
// @Description 手动触发调度请求参数
type TriggerScheduleRequest struct {
	PolicyID *uint `json:"policy_id,omitempty"` // 可选：指定使用的策略ID
}

// AutoSchedulerStatusResponse 自动调度器状态响应
// @Description 自动调度器状态信息
type AutoSchedulerStatusResponse struct {
	IsRunning           bool   `json:"is_running"`
	StartedAt           string `json:"started_at,omitempty"`
	LastScheduleAt      string `json:"last_schedule_at,omitempty"`
	TotalScheduled      int64  `json:"total_scheduled"`
	TotalFailed         int64  `json:"total_failed"`
	ActivePolicies      int    `json:"active_policies"`
	PendingTasks        int    `json:"pending_tasks"`
	AvailableBoxes      int    `json:"available_boxes"`
	LastError           string `json:"last_error,omitempty"`
	ScheduleIntervalSec int    `json:"schedule_interval_sec"`
}

// AutoScheduleResultResponse 自动调度结果响应
// @Description 自动调度执行结果
type AutoScheduleResultResponse struct {
	TotalTasks     int                         `json:"total_tasks"`
	ScheduledTasks int                         `json:"scheduled_tasks"`
	FailedTasks    int                         `json:"failed_tasks"`
	SkippedTasks   int                         `json:"skipped_tasks"`
	PolicyUsed     string                      `json:"policy_used"`
	DurationMs     int64                       `json:"duration_ms"`
	Details        []*service.TaskScheduleResult `json:"details,omitempty"`
	Errors         []string                    `json:"errors,omitempty"`
}

// GetAutoSchedulerStatus 获取自动调度器状态
// @Summary 获取自动调度器状态
// @Description 获取自动调度器的当前运行状态
// @Tags 自动调度
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=AutoSchedulerStatusResponse} "获取成功"
// @Router /api/v1/auto-scheduler/status [get]
// @Security ApiKeyAuth
func (c *AutoSchedulerController) GetAutoSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status := c.autoSchedulerService.GetStatus(ctx)

	response := &AutoSchedulerStatusResponse{
		IsRunning:           status.IsRunning,
		TotalScheduled:      status.TotalScheduled,
		TotalFailed:         status.TotalFailed,
		ActivePolicies:      status.ActivePolicies,
		PendingTasks:        status.PendingTasks,
		AvailableBoxes:      status.AvailableBoxes,
		LastError:           status.LastError,
		ScheduleIntervalSec: status.ScheduleIntervalSec,
	}

	if status.StartedAt != nil {
		response.StartedAt = status.StartedAt.Format("2006-01-02 15:04:05")
	}
	if status.LastScheduleAt != nil {
		response.LastScheduleAt = status.LastScheduleAt.Format("2006-01-02 15:04:05")
	}

	render.Render(w, r, SuccessResponse("获取自动调度器状态成功", response))
}

// StartAutoScheduler 启动自动调度器
// @Summary 启动自动调度器
// @Description 启动自动调度器，开始定时调度任务
// @Tags 自动调度
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse "启动成功"
// @Failure 400 {object} ErrorResponse "调度器已在运行"
// @Failure 500 {object} ErrorResponse "启动失败"
// @Router /api/v1/auto-scheduler/start [post]
// @Security ApiKeyAuth
func (c *AutoSchedulerController) StartAutoScheduler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if c.autoSchedulerService.IsRunning() {
		render.Render(w, r, BadRequestResponse("自动调度器已在运行", nil))
		return
	}

	if err := c.autoSchedulerService.Start(ctx); err != nil {
		render.Render(w, r, InternalErrorResponse("启动自动调度器失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("自动调度器已启动", nil))
}

// StopAutoScheduler 停止自动调度器
// @Summary 停止自动调度器
// @Description 停止自动调度器
// @Tags 自动调度
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse "停止成功"
// @Failure 400 {object} ErrorResponse "调度器未运行"
// @Failure 500 {object} ErrorResponse "停止失败"
// @Router /api/v1/auto-scheduler/stop [post]
// @Security ApiKeyAuth
func (c *AutoSchedulerController) StopAutoScheduler(w http.ResponseWriter, r *http.Request) {
	if !c.autoSchedulerService.IsRunning() {
		render.Render(w, r, BadRequestResponse("自动调度器未运行", nil))
		return
	}

	if err := c.autoSchedulerService.Stop(); err != nil {
		render.Render(w, r, InternalErrorResponse("停止自动调度器失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("自动调度器已停止", nil))
}

// TriggerSchedule 手动触发调度
// @Summary 手动触发调度
// @Description 手动触发一次调度，可选择指定策略
// @Tags 自动调度
// @Accept json
// @Produce json
// @Param request body TriggerScheduleRequest false "触发请求"
// @Success 200 {object} APIResponse{data=AutoScheduleResultResponse} "调度完成"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "调度失败"
// @Router /api/v1/auto-scheduler/trigger [post]
// @Security ApiKeyAuth
func (c *AutoSchedulerController) TriggerSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req TriggerScheduleRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			render.Render(w, r, BadRequestResponse("无效的请求数据", err))
			return
		}
	}

	var result *service.AutoScheduleResult
	var err error

	if req.PolicyID != nil {
		result, err = c.autoSchedulerService.TriggerScheduleWithPolicy(ctx, *req.PolicyID)
	} else {
		result, err = c.autoSchedulerService.TriggerSchedule(ctx)
	}

	if err != nil {
		render.Render(w, r, InternalErrorResponse("调度执行失败", err))
		return
	}

	response := &AutoScheduleResultResponse{
		TotalTasks:     result.TotalTasks,
		ScheduledTasks: result.ScheduledTasks,
		FailedTasks:    result.FailedTasks,
		SkippedTasks:   result.SkippedTasks,
		PolicyUsed:     result.PolicyUsed,
		DurationMs:     result.Duration.Milliseconds(),
		Details:        result.Details,
		Errors:         result.Errors,
	}

	render.Render(w, r, SuccessResponse("调度执行完成", response))
}

// TriggerScheduleWithPolicy 使用指定策略触发调度
// @Summary 使用指定策略触发调度
// @Description 使用指定的调度策略触发一次调度
// @Tags 自动调度
// @Accept json
// @Produce json
// @Param id path int true "策略ID"
// @Success 200 {object} APIResponse{data=AutoScheduleResultResponse} "调度完成"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "策略不存在"
// @Failure 500 {object} ErrorResponse "调度失败"
// @Router /api/v1/auto-scheduler/trigger/{id} [post]
// @Security ApiKeyAuth
func (c *AutoSchedulerController) TriggerScheduleWithPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的策略ID", err))
		return
	}

	result, err := c.autoSchedulerService.TriggerScheduleWithPolicy(ctx, uint(id))
	if err != nil {
		render.Render(w, r, InternalErrorResponse("调度执行失败", err))
		return
	}

	response := &AutoScheduleResultResponse{
		TotalTasks:     result.TotalTasks,
		ScheduledTasks: result.ScheduledTasks,
		FailedTasks:    result.FailedTasks,
		SkippedTasks:   result.SkippedTasks,
		PolicyUsed:     result.PolicyUsed,
		DurationMs:     result.Duration.Milliseconds(),
		Details:        result.Details,
		Errors:         result.Errors,
	}

	render.Render(w, r, SuccessResponse("调度执行完成", response))
}

// RegisterRoutes 注册路由
func (c *AutoSchedulerController) RegisterRoutes(r chi.Router) {
	r.Route("/auto-scheduler", func(r chi.Router) {
		r.Get("/status", c.GetAutoSchedulerStatus)
		r.Post("/start", c.StartAutoScheduler)
		r.Post("/stop", c.StopAutoScheduler)
		r.Post("/trigger", c.TriggerSchedule)
		r.Post("/trigger/{id}", c.TriggerScheduleWithPolicy)
	})
}

