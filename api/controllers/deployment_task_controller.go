/*
 * @module api/controllers/deployment_task_controller
 * @description 部署任务管理控制器，提供部署任务的创建、监控、控制等功能
 * @architecture MVC架构 - 控制器层
 * @documentReference REQ-004: 任务调度系统
 * @stateFlow HTTP请求处理 -> 业务逻辑处理 -> 数据库操作 -> 响应返回
 * @rules 遵循PostgREST RBAC权限验证，所有操作需要相应权限
 * @dependencies box-manage-service/service
 * @refs DESIGN-000.md
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/service"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// DeploymentTaskController 部署任务控制器
type DeploymentTaskController struct {
	deploymentService service.DeploymentTaskService
}

// NewDeploymentTaskController 创建部署任务控制器实例
func NewDeploymentTaskController(deploymentService service.DeploymentTaskService) *DeploymentTaskController {
	return &DeploymentTaskController{
		deploymentService: deploymentService,
	}
}

// CreateDeploymentTaskRequest 创建部署任务请求
type CreateDeploymentTaskRequest struct {
	Name             string                   `json:"name" validate:"required"`
	Description      string                   `json:"description"`
	TaskIDs          []uint                   `json:"task_ids" validate:"required,min=1"`
	TargetBoxIDs     []uint                   `json:"target_box_ids" validate:"required,min=1"`
	Priority         int                      `json:"priority"`
	DeploymentConfig *models.DeploymentConfig `json:"deployment_config"`
	AutoStart        bool                     `json:"auto_start"` // 是否创建后自动开始
}

// BatchDeploymentRequest 批量部署请求
type BatchDeploymentRequest struct {
	TaskIDs          []uint                   `json:"task_ids" validate:"required,min=1"`
	TargetBoxIDs     []uint                   `json:"target_box_ids" validate:"required,min=1"`
	DeploymentConfig *models.DeploymentConfig `json:"deployment_config"`
	AutoStart        bool                     `json:"auto_start"`
}

// CreateDeploymentTask 创建部署任务
// @Summary 创建部署任务
// @Description 创建一个新的部署任务，用于管理单个或多个任务的部署过程
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param deployment body CreateDeploymentTaskRequest true "部署任务配置"
// @Success 200 {object} APIResponse{data=models.DeploymentTask} "部署任务创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments [post]
func (c *DeploymentTaskController) CreateDeploymentTask(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DeploymentTaskController] CreateDeploymentTask request received from %s", r.RemoteAddr)

	var req CreateDeploymentTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[DeploymentTaskController] Failed to decode request - Error: %v", err)
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	log.Printf("[DeploymentTaskController] Creating deployment task - Name: %s, Tasks: %d, Boxes: %d",
		req.Name, len(req.TaskIDs), len(req.TargetBoxIDs))

	// 创建服务请求
	serviceReq := &service.CreateDeploymentTaskRequest{
		Name:             req.Name,
		Description:      req.Description,
		TaskIDs:          req.TaskIDs,
		TargetBoxIDs:     req.TargetBoxIDs,
		Priority:         req.Priority,
		DeploymentConfig: req.DeploymentConfig,
		CreatedBy:        c.getCurrentUserID(r),
	}

	// 创建部署任务
	deploymentTask, err := c.deploymentService.CreateDeploymentTask(r.Context(), serviceReq)
	if err != nil {
		log.Printf("[DeploymentTaskController] Failed to create deployment task - Error: %v", err)
		render.Render(w, r, InternalErrorResponse("创建部署任务失败", err))
		return
	}

	// 如果设置了自动开始，立即启动
	if req.AutoStart {
		if err := c.deploymentService.StartDeploymentTask(r.Context(), deploymentTask.ID); err != nil {
			log.Printf("[DeploymentTaskController] Failed to auto-start deployment task - Error: %v", err)
			// 不返回错误，部署任务已创建成功
		} else {
			log.Printf("[DeploymentTaskController] Deployment task %d auto-started", deploymentTask.ID)
		}
	}

	log.Printf("[DeploymentTaskController] Deployment task created successfully - ID: %d", deploymentTask.ID)
	render.Render(w, r, SuccessResponse("部署任务创建成功", deploymentTask))
}

// GetDeploymentTasks 获取部署任务列表
// @Summary 获取部署任务列表
// @Description 获取部署任务列表，支持按状态等条件筛选
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param status query string false "部署状态筛选" Enums(pending,running,completed,failed,cancelled)
// @Param created_by query int false "创建者用户ID"
// @Param priority query int false "优先级筛选"
// @Param keyword query string false "关键词搜索"
// @Success 200 {object} APIResponse{data=[]models.DeploymentTask} "获取部署任务列表成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments [get]
func (c *DeploymentTaskController) GetDeploymentTasks(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DeploymentTaskController] GetDeploymentTasks request received")

	// 解析查询参数
	filters := &service.DeploymentTaskFilters{
		Keyword: r.URL.Query().Get("keyword"),
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := models.DeploymentTaskStatus(status)
		filters.Status = &s
	}

	if createdByStr := r.URL.Query().Get("created_by"); createdByStr != "" {
		if createdBy, err := strconv.ParseUint(createdByStr, 10, 32); err == nil {
			uid := uint(createdBy)
			filters.CreatedBy = &uid
		}
	}

	if priorityStr := r.URL.Query().Get("priority"); priorityStr != "" {
		if priority, err := strconv.Atoi(priorityStr); err == nil {
			filters.Priority = &priority
		}
	}

	// 获取部署任务列表
	deploymentTasks, err := c.deploymentService.GetDeploymentTasks(r.Context(), filters)
	if err != nil {
		log.Printf("[DeploymentTaskController] Failed to get deployment tasks - Error: %v", err)
		render.Render(w, r, InternalErrorResponse("获取部署任务列表失败", err))
		return
	}

	log.Printf("[DeploymentTaskController] Found %d deployment tasks", len(deploymentTasks))
	render.Render(w, r, SuccessResponse("获取部署任务列表成功", deploymentTasks))
}

// GetDeploymentTask 获取部署任务详情
// @Summary 获取部署任务详情
// @Description 根据ID获取部署任务的详细信息
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param id path int true "部署任务ID"
// @Success 200 {object} APIResponse{data=models.DeploymentTask} "获取部署任务详情成功"
// @Failure 400 {object} APIResponse "部署任务ID不能为空"
// @Failure 404 {object} APIResponse "部署任务不存在"
// @Router /api/v1/deployments/{id} [get]
func (c *DeploymentTaskController) GetDeploymentTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		render.Render(w, r, BadRequestResponse("部署任务ID不能为空", nil))
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("部署任务ID格式错误", err))
		return
	}

	deploymentTask, err := c.deploymentService.GetDeploymentTask(r.Context(), uint(id))
	if err != nil {
		render.Render(w, r, NotFoundResponse("部署任务不存在", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取部署任务详情成功", deploymentTask))
}

// StartDeploymentTask 开始执行部署任务
// @Summary 开始执行部署任务
// @Description 启动指定的部署任务
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param id path int true "部署任务ID"
// @Success 200 {object} APIResponse "部署任务启动成功"
// @Failure 400 {object} APIResponse "请求参数错误或任务状态不允许启动"
// @Failure 404 {object} APIResponse "部署任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments/{id}/start [post]
func (c *DeploymentTaskController) StartDeploymentTask(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseIDParam(r)
	if err != nil {
		render.Render(w, r, BadRequestResponse("部署任务ID格式错误", err))
		return
	}

	log.Printf("[DeploymentTaskController] Starting deployment task: %d", id)

	if err := c.deploymentService.StartDeploymentTask(r.Context(), id); err != nil {
		log.Printf("[DeploymentTaskController] Failed to start deployment task %d - Error: %v", id, err)
		render.Render(w, r, InternalErrorResponse("启动部署任务失败", err))
		return
	}

	log.Printf("[DeploymentTaskController] Deployment task %d started successfully", id)
	render.Render(w, r, SuccessResponse("部署任务启动成功", nil))
}

// CancelDeploymentTask 取消部署任务
// @Summary 取消部署任务
// @Description 取消指定的部署任务
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param id path int true "部署任务ID"
// @Param cancel body map[string]string false "取消原因" example({"reason": "用户取消"})
// @Success 200 {object} APIResponse "部署任务取消成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "部署任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments/{id}/cancel [post]
func (c *DeploymentTaskController) CancelDeploymentTask(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseIDParam(r)
	if err != nil {
		render.Render(w, r, BadRequestResponse("部署任务ID格式错误", err))
		return
	}

	// 解析取消原因
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Reason = "用户取消"
	}

	log.Printf("[DeploymentTaskController] Cancelling deployment task: %d, reason: %s", id, req.Reason)

	if err := c.deploymentService.CancelDeploymentTask(r.Context(), id, req.Reason); err != nil {
		log.Printf("[DeploymentTaskController] Failed to cancel deployment task %d - Error: %v", id, err)
		render.Render(w, r, InternalErrorResponse("取消部署任务失败", err))
		return
	}

	log.Printf("[DeploymentTaskController] Deployment task %d cancelled successfully", id)
	render.Render(w, r, SuccessResponse("部署任务取消成功", nil))
}

// GetDeploymentTaskLogs 获取部署任务日志
// @Summary 获取部署任务日志
// @Description 获取指定部署任务的执行日志
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param id path int true "部署任务ID"
// @Param limit query int false "日志条数限制" default(100)
// @Success 200 {object} APIResponse{data=[]models.DeploymentLog} "获取部署任务日志成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "部署任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments/{id}/logs [get]
func (c *DeploymentTaskController) GetDeploymentTaskLogs(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseIDParam(r)
	if err != nil {
		render.Render(w, r, BadRequestResponse("部署任务ID格式错误", err))
		return
	}

	// 解析limit参数
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	logs, err := c.deploymentService.GetDeploymentTaskLogs(r.Context(), id, limit)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取部署任务日志失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取部署任务日志成功", logs))
}

// GetDeploymentTaskResults 获取部署任务结果
// @Summary 获取部署任务结果
// @Description 获取指定部署任务的执行结果
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param id path int true "部署任务ID"
// @Success 200 {object} APIResponse{data=[]models.DeploymentResult} "获取部署任务结果成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "部署任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments/{id}/results [get]
func (c *DeploymentTaskController) GetDeploymentTaskResults(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseIDParam(r)
	if err != nil {
		render.Render(w, r, BadRequestResponse("部署任务ID格式错误", err))
		return
	}

	results, err := c.deploymentService.GetDeploymentTaskResults(r.Context(), id)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取部署任务结果失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取部署任务结果成功", results))
}

// GetDeploymentTaskProgress 获取部署任务进度
// @Summary 获取部署任务进度
// @Description 获取指定部署任务的执行进度
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param id path int true "部署任务ID"
// @Success 200 {object} APIResponse{data=service.DeploymentProgress} "获取部署任务进度成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "部署任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments/{id}/progress [get]
func (c *DeploymentTaskController) GetDeploymentTaskProgress(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseIDParam(r)
	if err != nil {
		render.Render(w, r, BadRequestResponse("部署任务ID格式错误", err))
		return
	}

	progress, err := c.deploymentService.GetDeploymentTaskProgress(r.Context(), id)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取部署任务进度失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取部署任务进度成功", progress))
}

// DeleteDeploymentTask 删除部署任务
// @Summary 删除部署任务
// @Description 删除指定的部署任务，如果任务正在运行会先取消
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param id path int true "部署任务ID"
// @Success 200 {object} APIResponse "部署任务删除成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 404 {object} APIResponse "部署任务不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments/{id} [delete]
func (c *DeploymentTaskController) DeleteDeploymentTask(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseIDParam(r)
	if err != nil {
		render.Render(w, r, BadRequestResponse("部署任务ID格式错误", err))
		return
	}

	log.Printf("[DeploymentTaskController] Deleting deployment task: %d", id)

	if err := c.deploymentService.DeleteDeploymentTask(r.Context(), id); err != nil {
		log.Printf("[DeploymentTaskController] Failed to delete deployment task %d - Error: %v", id, err)
		render.Render(w, r, InternalErrorResponse("删除部署任务失败", err))
		return
	}

	log.Printf("[DeploymentTaskController] Deployment task %d deleted successfully", id)
	render.Render(w, r, SuccessResponse("部署任务删除成功", nil))
}

// BatchCreateDeployment 批量创建部署
// @Summary 批量创建部署
// @Description 批量创建部署任务，将多个任务部署到多个盒子
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param batch body BatchDeploymentRequest true "批量部署配置"
// @Success 200 {object} APIResponse{data=models.DeploymentTask} "批量部署创建成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments/batch [post]
func (c *DeploymentTaskController) BatchCreateDeployment(w http.ResponseWriter, r *http.Request) {
	var req BatchDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数格式错误", err))
		return
	}

	log.Printf("[DeploymentTaskController] Creating batch deployment - Tasks: %d, Boxes: %d",
		len(req.TaskIDs), len(req.TargetBoxIDs))

	deploymentTask, err := c.deploymentService.BatchCreateDeployment(r.Context(), req.TaskIDs, req.TargetBoxIDs, req.DeploymentConfig)
	if err != nil {
		log.Printf("[DeploymentTaskController] Failed to create batch deployment - Error: %v", err)
		render.Render(w, r, InternalErrorResponse("创建批量部署失败", err))
		return
	}

	// 如果设置了自动开始，立即启动
	if req.AutoStart {
		if err := c.deploymentService.StartDeploymentTask(r.Context(), deploymentTask.ID); err != nil {
			log.Printf("[DeploymentTaskController] Failed to auto-start batch deployment - Error: %v", err)
		}
	}

	log.Printf("[DeploymentTaskController] Batch deployment created successfully - ID: %d", deploymentTask.ID)
	render.Render(w, r, SuccessResponse("批量部署创建成功", deploymentTask))
}

// GetDeploymentStatistics 获取部署统计
// @Summary 获取部署统计
// @Description 获取部署任务的统计信息
// @Tags 部署管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse "获取部署统计成功"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments/statistics [get]
func (c *DeploymentTaskController) GetDeploymentStatistics(w http.ResponseWriter, r *http.Request) {
	statistics, err := c.deploymentService.GetDeploymentStatistics(r.Context())
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取部署统计失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("获取部署统计成功", statistics))
}

// CleanupOldTasks 清理旧任务
// @Summary 清理旧任务
// @Description 清理指定时间之前完成的部署任务
// @Tags 部署管理
// @Accept json
// @Produce json
// @Param days query int false "保留天数" default(30)
// @Success 200 {object} APIResponse "清理任务成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/deployments/cleanup [post]
func (c *DeploymentTaskController) CleanupOldTasks(w http.ResponseWriter, r *http.Request) {
	// 解析保留天数
	days := 30
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if parsedDays, err := strconv.Atoi(daysStr); err == nil && parsedDays > 0 {
			days = parsedDays
		}
	}

	olderThan := time.Now().AddDate(0, 0, -days)
	log.Printf("[DeploymentTaskController] Cleaning up deployment tasks older than %v", olderThan)

	count, err := c.deploymentService.CleanupOldTasks(r.Context(), olderThan)
	if err != nil {
		log.Printf("[DeploymentTaskController] Failed to cleanup old tasks - Error: %v", err)
		render.Render(w, r, InternalErrorResponse("清理旧任务失败", err))
		return
	}

	log.Printf("[DeploymentTaskController] Cleaned up %d old deployment tasks", count)
	render.Render(w, r, SuccessResponse("清理任务成功", map[string]interface{}{
		"deleted_count": count,
		"older_than":    olderThan,
	}))
}

// 辅助方法

// parseIDParam 解析ID参数
func (c *DeploymentTaskController) parseIDParam(r *http.Request) (uint, error) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		return 0, fmt.Errorf("部署任务ID不能为空")
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("部署任务ID格式错误")
	}

	return uint(id), nil
}

// getCurrentUserID 从请求上下文获取当前用户ID
func (c *DeploymentTaskController) getCurrentUserID(r *http.Request) uint {
	return getUserIDFromRequest(r)
}
