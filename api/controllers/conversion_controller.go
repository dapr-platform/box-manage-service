/*
 * @module controllers/conversion_controller
 * @description 模型转换API控制器，提供转换任务的HTTP接口
 * @architecture 控制器层
 * @documentReference REQ-003: 模型转换功能
 * @stateFlow HTTP请求 -> 参数验证 -> Service调用 -> 响应返回
 * @rules 提供RESTful风格的模型转换任务管理接口
 * @dependencies service.ConversionService, go-chi/render
 * @refs REQ-003.md, DESIGN-005.md
 */

package controllers

import (
	"box-manage-service/service"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// ConversionController 转换控制器
type ConversionController struct {
	conversionService service.ConversionService
}

// NewConversionController 创建转换控制器
func NewConversionController(conversionService service.ConversionService) *ConversionController {
	return &ConversionController{
		conversionService: conversionService,
	}
}

// CreateConversionTask 创建转换任务
// @Summary 创建转换任务
// @Description 创建新的模型转换任务
// @Tags 模型转换
// @Accept json
// @Produce json
// @Param request body service.CreateConversionTaskRequest true "转换任务信息"
// @Success 201 {object} APIResponse{data=[]models.ConversionTask}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/conversion/tasks [post]
func (c *ConversionController) CreateConversionTask(w http.ResponseWriter, r *http.Request) {
	var req service.CreateConversionTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	// 设置创建用户（从请求头或上下文获取）
	req.CreatedBy = c.getCurrentUserID(r)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	tasks, err := c.conversionService.CreateConversionTask(ctx, &req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("创建转换任务失败", err))
		return
	}

	// 构建响应消息
	message := "转换任务创建成功"
	if req.AutoStart {
		message = fmt.Sprintf("转换任务创建成功，共 %d 个任务，正在后台自动启动", len(tasks))
	} else {
		message = fmt.Sprintf("转换任务创建成功，共 %d 个任务", len(tasks))
	}

	render.Status(r, http.StatusCreated)
	render.Render(w, r, SuccessResponse(message, tasks))
}

// GetConversionTasks 获取转换任务列表
// @Summary 获取转换任务列表
// @Description 分页获取转换任务列表，支持多条件搜索
// @Tags 模型转换
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param status query string false "任务状态" Enums(pending,running,completed,failed)
// @Param user_id query int false "用户ID"
// @Param original_model_id query int false "原始模型ID"
// @Param keyword query string false "搜索关键词"
// @Param target_chip query string false "目标芯片" Enums(bm1684,bm1684x,bm1688)
// @Param start_time query string false "开始时间（RFC3339格式）"
// @Param end_time query string false "结束时间（RFC3339格式）"
// @Success 200 {object} PaginatedResponse{data=[]models.ConversionTask}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/conversion/tasks [get]
func (c *ConversionController) GetConversionTasks(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	page := parseIntWithDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntWithDefault(r.URL.Query().Get("page_size"), 20)
	status := r.URL.Query().Get("status")
	keyword := r.URL.Query().Get("keyword")
	targetChip := r.URL.Query().Get("target_chip")

	var userID *uint
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			u := uint(uid)
			userID = &u
		}
	}

	var originalModelID *uint
	if modelIDStr := r.URL.Query().Get("original_model_id"); modelIDStr != "" {
		if mid, err := strconv.ParseUint(modelIDStr, 10, 32); err == nil {
			m := uint(mid)
			originalModelID = &m
		}
	}

	// 解析时间参数
	var startTime, endTime time.Time
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

	req := &service.GetConversionTasksRequest{
		Page:            page,
		PageSize:        pageSize,
		Status:          status,
		UserID:          userID,
		OriginalModelID: originalModelID,
		StartTime:       startTime,
		EndTime:         endTime,
		Keyword:         keyword,
		// TODO: 需要在service.GetConversionTasksRequest中添加TargetChip字段的支持
		// TargetChip:      targetChip,
	}

	// TODO: 暂时忽略targetChip参数，后续需要在service层实现
	_ = targetChip

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := c.conversionService.GetConversionTasks(ctx, req)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取转换任务列表失败", err))
		return
	}

	render.Render(w, r, PaginatedSuccessResponse(
		"获取转换任务列表成功",
		response.Tasks,
		response.Total,
		response.Page,
		response.PageSize,
	))
}

// GetConversionTask 获取转换任务详情
// @Summary 获取转换任务详情
// @Description 根据任务ID获取转换任务详情
// @Tags 模型转换
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse{data=models.ConversionTask}
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/conversion/tasks/{taskId} [get]
func (c *ConversionController) GetConversionTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	task, err := c.conversionService.GetConversionTask(ctx, taskID)
	if err != nil {
		render.Render(w, r, NotFoundResponse("转换任务不存在", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取转换任务详情成功", task))
}

// DeleteConversionTask 删除转换任务
// @Summary 删除转换任务
// @Description 删除指定的转换任务
// @Tags 模型转换
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/conversion/tasks/{taskId} [delete]
func (c *ConversionController) DeleteConversionTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := c.conversionService.DeleteConversionTask(ctx, taskID); err != nil {
		render.Render(w, r, InternalErrorResponse("删除转换任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("删除转换任务成功", nil))
}

// StartConversion 手动启动转换
// @Summary 手动启动转换
// @Description 手动启动指定的转换任务
// @Tags 模型转换
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/conversion/tasks/{taskId}/start [post]
func (c *ConversionController) StartConversion(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := c.conversionService.StartConversion(ctx, taskID); err != nil {
		render.Render(w, r, InternalErrorResponse("启动转换失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("转换任务启动成功", nil))
}

// StopConversion 停止转换
// @Summary 停止转换
// @Description 停止正在运行的转换任务
// @Tags 模型转换
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/conversion/tasks/{taskId}/stop [post]
func (c *ConversionController) StopConversion(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := c.conversionService.StopConversion(ctx, taskID); err != nil {
		render.Render(w, r, InternalErrorResponse("停止转换失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("转换任务停止成功", nil))
}

// GetConversionProgress 获取转换进度
// @Summary 获取转换进度
// @Description 获取转换任务的当前进度
// @Tags 模型转换
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse{data=service.ConversionProgress}
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/conversion/tasks/{taskId}/progress [get]
func (c *ConversionController) GetConversionProgress(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	progress, err := c.conversionService.GetConversionProgress(ctx, taskID)
	if err != nil {
		render.Render(w, r, NotFoundResponse("获取转换进度失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取转换进度成功", progress))
}

// GetConversionLogs 获取转换日志
// @Summary 获取转换日志
// @Description 获取转换任务的执行日志
// @Tags 模型转换
// @Accept json
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} APIResponse{data=[]string}
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/conversion/tasks/{taskId}/logs [get]
func (c *ConversionController) GetConversionLogs(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		render.Render(w, r, BadRequestResponse("任务ID不能为空", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	logs, err := c.conversionService.GetConversionLogs(ctx, taskID)
	if err != nil {
		render.Render(w, r, NotFoundResponse("获取转换日志失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取转换日志成功", logs))
}

// GetConversionStatistics 获取转换统计
// @Summary 获取转换统计
// @Description 获取转换任务的统计信息
// @Tags 模型转换
// @Accept json
// @Produce json
// @Param user_id query int false "用户ID"
// @Success 200 {object} APIResponse{data=service.ConversionStatistics}
// @Failure 500 {object} ErrorResponse
// @Router /api/conversion/statistics [get]
func (c *ConversionController) GetConversionStatistics(w http.ResponseWriter, r *http.Request) {
	var userID *uint
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			u := uint(uid)
			userID = &u
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	statistics, err := c.conversionService.GetConversionStatistics(ctx, userID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取转换统计失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取转换统计成功", statistics))
}

// getCurrentUserID 获取当前用户ID（从请求头或上下文获取）
func (c *ConversionController) getCurrentUserID(r *http.Request) uint {
	// 这里应该从JWT token或session中获取用户ID
	// 为了简化，返回固定值1
	return 1
}
