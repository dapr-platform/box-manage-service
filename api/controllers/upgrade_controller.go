/*
 * @module api/controllers/upgrade_controller
 * @description 升级管理控制器，提供盒子软件升级、回滚、进度跟踪等功能
 * @architecture MVC架构 - 控制器层
 * @documentReference REQ-001: 盒子管理功能
 * @stateFlow HTTP请求处理 -> 升级服务调用 -> 进度跟踪 -> 响应返回
 * @rules 支持单个和批量升级、版本回滚、进度监控等功能
 * @dependencies box-manage-service/service
 * @refs DESIGN-001.md
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/service"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// UpgradeController 升级管理控制器
type UpgradeController struct {
	upgradeService *service.UpgradeService
}

// NewUpgradeController 创建升级控制器实例
func NewUpgradeController(upgradeService *service.UpgradeService) *UpgradeController {
	return &UpgradeController{
		upgradeService: upgradeService,
	}
}

// 请求/响应结构体定义

// UpgradeRequest 升级请求
type UpgradeRequest struct {
	Version     string `json:"version" binding:"required" example:"1.0.1"`
	ProgramFile string `json:"program_file" binding:"required" example:"/path/to/program.tar.gz"`
	Force       bool   `json:"force" example:"false"`
}

// BatchUpgradeRequest 批量升级请求
type BatchUpgradeRequest struct {
	BoxIDs      []uint `json:"box_ids" binding:"required"`
	Version     string `json:"version" binding:"required" example:"1.0.1"`
	ProgramFile string `json:"program_file" binding:"required" example:"/path/to/program.tar.gz"`
	Force       bool   `json:"force" example:"false"`
}

// RollbackRequest 回滚请求
type RollbackRequest struct {
	TargetVersion string `json:"target_version" binding:"required" example:"1.0.0"`
}

// UpgradeTaskResponse 升级任务响应
type UpgradeTaskResponse struct {
	ID              uint                 `json:"id"`
	BoxID           uint                 `json:"box_id"`
	BoxName         string               `json:"box_name"`
	Name            string               `json:"name"`
	VersionFrom     string               `json:"version_from"`
	VersionTo       string               `json:"version_to"`
	Status          models.UpgradeStatus `json:"status"`
	Progress        int                  `json:"progress"`
	ProgramFile     string               `json:"program_file"`
	Force           bool                 `json:"force"`
	StartedAt       *string              `json:"started_at"`
	CompletedAt     *string              `json:"completed_at"`
	ErrorMessage    string               `json:"error_message"`
	CanRollback     bool                 `json:"can_rollback"`
	RollbackVersion string               `json:"rollback_version"`
	CreatedAt       string               `json:"created_at"`
}

// BatchUpgradeTaskResponse 批量升级任务响应
type BatchUpgradeTaskResponse struct {
	ID             uint                 `json:"id"`
	Name           string               `json:"name"`
	BoxIDs         []uint               `json:"box_ids"`
	VersionTo      string               `json:"version_to"`
	Status         models.UpgradeStatus `json:"status"`
	TotalBoxes     int                  `json:"total_boxes"`
	CompletedBoxes int                  `json:"completed_boxes"`
	FailedBoxes    int                  `json:"failed_boxes"`
	ProgramFile    string               `json:"program_file"`
	Force          bool                 `json:"force"`
	StartedAt      *string              `json:"started_at"`
	CompletedAt    *string              `json:"completed_at"`
	CreatedAt      string               `json:"created_at"`
}

// 单个盒子升级API

// UpgradeBox 升级单个盒子
// @Summary 升级单个盒子
// @Description 对指定盒子执行软件升级
// @Tags 升级管理
// @Accept json
// @Produce json
// @Param id path int true "盒子ID"
// @Param request body UpgradeRequest true "升级请求"
// @Success 200 {object} APIResponse{data=UpgradeTaskResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id}/upgrade [post]
func (c *UpgradeController) UpgradeBox(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	var req UpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数错误", err))
		return
	}

	boxID := uint(id)

	// 获取当前用户ID
	createdBy := c.getCurrentUserID(r)

	// 创建升级任务
	task, err := c.upgradeService.CreateUpgradeTask(boxID, req.Version, req.ProgramFile, req.Force, createdBy)
	if err != nil {
		render.Render(w, r, BadRequestResponse("创建升级任务失败", err))
		return
	}

	// 执行升级
	if err := c.upgradeService.ExecuteUpgrade(task.ID); err != nil {
		render.Render(w, r, InternalErrorResponse("启动升级失败", err))
		return
	}

	// 转换响应格式
	response := convertUpgradeTaskToResponse(task)

	render.Render(w, r, SuccessResponse("升级任务已创建并开始执行", response))
}

// RollbackBox 回滚盒子版本
// @Summary 回滚盒子版本
// @Description 将指定盒子回滚到之前的版本
// @Tags 升级管理
// @Accept json
// @Produce json
// @Param id path int true "盒子ID"
// @Param request body RollbackRequest true "回滚请求"
// @Success 200 {object} APIResponse{data=UpgradeTaskResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/boxes/{id}/rollback [post]
func (c *UpgradeController) RollbackBox(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的盒子ID", err))
		return
	}

	var req RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数错误", err))
		return
	}

	boxID := uint(id)

	// 获取当前用户ID
	createdBy := c.getCurrentUserID(r)

	// 创建回滚任务
	task, err := c.upgradeService.RollbackVersion(boxID, req.TargetVersion, createdBy)
	if err != nil {
		render.Render(w, r, BadRequestResponse("创建回滚任务失败", err))
		return
	}

	// 执行回滚
	if err := c.upgradeService.ExecuteUpgrade(task.ID); err != nil {
		render.Render(w, r, InternalErrorResponse("启动回滚失败", err))
		return
	}

	// 转换响应格式
	response := convertUpgradeTaskToResponse(task)

	render.Render(w, r, SuccessResponse("回滚任务已创建并开始执行", response))
}

// 批量升级API

// BatchUpgrade 批量升级盒子
// @Summary 批量升级盒子
// @Description 对多个盒子执行批量软件升级
// @Tags 升级管理
// @Accept json
// @Produce json
// @Param request body BatchUpgradeRequest true "批量升级请求"
// @Success 200 {object} APIResponse{data=BatchUpgradeTaskResponse}
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/boxes/batch/upgrade [post]
func (c *UpgradeController) BatchUpgrade(w http.ResponseWriter, r *http.Request) {
	var req BatchUpgradeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Render(w, r, BadRequestResponse("请求参数错误", err))
		return
	}

	if len(req.BoxIDs) == 0 {
		render.Render(w, r, BadRequestResponse("盒子ID列表不能为空", nil))
		return
	}

	// 获取当前用户ID
	createdBy := c.getCurrentUserID(r)

	// 创建批量升级任务
	batchTask, err := c.upgradeService.CreateBatchUpgradeTask(req.BoxIDs, req.Version, req.ProgramFile, req.Force, createdBy)
	if err != nil {
		render.Render(w, r, BadRequestResponse("创建批量升级任务失败", err))
		return
	}

	// 转换响应格式
	response := convertBatchUpgradeTaskToResponse(batchTask)

	render.Render(w, r, SuccessResponse("批量升级任务已创建", response))
}

// 升级任务管理API

// GetUpgradeTasks 获取升级任务列表
// @Summary 获取升级任务列表
// @Description 获取升级任务列表，支持筛选和分页
// @Tags 升级管理
// @Produce json
// @Param box_id query int false "盒子ID"
// @Param status query string false "任务状态" Enums(pending,running,completed,failed,cancelled)
// @Param version_to query string false "目标版本号"
// @Param version_from query string false "源版本号"
// @Param name query string false "任务名称搜索关键词"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Success 200 {object} PaginatedResponse{data=[]UpgradeTaskResponse}
// @Router /api/v1/upgrades [get]
func (c *UpgradeController) GetUpgradeTasks(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	var boxID *uint
	if boxIDStr := r.URL.Query().Get("box_id"); boxIDStr != "" {
		if id, err := strconv.ParseUint(boxIDStr, 10, 32); err == nil {
			boxIDValue := uint(id)
			boxID = &boxIDValue
		}
	}

	var status *models.UpgradeStatus
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		statusValue := models.UpgradeStatus(statusStr)
		status = &statusValue
	}

	// TODO: 需要在service层添加对版本号和名称搜索的支持
	// versionTo := r.URL.Query().Get("version_to")
	// versionFrom := r.URL.Query().Get("version_from")
	// name := r.URL.Query().Get("name")

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 {
		size = 10
	}

	offset := (page - 1) * size

	// 获取升级任务列表
	tasks, total, err := c.upgradeService.GetUpgradeTasks(boxID, status, size, offset)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("获取升级任务列表失败", err))
		return
	}

	// 转换响应格式
	var responses []UpgradeTaskResponse
	for _, task := range tasks {
		responses = append(responses, *convertUpgradeTaskToResponse(task))
	}

	render.Render(w, r, PaginatedSuccessResponse("获取升级任务列表成功", responses, total, page, size))
}

// GetUpgradeTask 获取升级任务详情
// @Summary 获取升级任务详情
// @Description 获取指定升级任务的详细信息
// @Tags 升级管理
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse{data=UpgradeTaskResponse}
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/upgrades/{id} [get]
func (c *UpgradeController) GetUpgradeTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	taskID := uint(id)

	// 获取升级任务详情
	task, err := c.upgradeService.TrackProgress(taskID)
	if err != nil {
		render.Render(w, r, NotFoundResponse("升级任务不存在", err))
		return
	}

	// 转换响应格式
	response := convertUpgradeTaskToResponse(task)

	render.Render(w, r, SuccessResponse("获取升级任务详情成功", response))
}

// CancelUpgradeTask 取消升级任务
// @Summary 取消升级任务
// @Description 取消指定的升级任务（仅限待执行或执行中的任务）
// @Tags 升级管理
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/upgrades/{id}/cancel [post]
func (c *UpgradeController) CancelUpgradeTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	taskID := uint(id)

	// 取消升级任务
	err = c.upgradeService.CancelUpgradeTask(taskID)
	if err != nil {
		render.Render(w, r, BadRequestResponse("取消升级任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("升级任务已取消", nil))
}

// RetryUpgradeTask 重试升级任务
// @Summary 重试升级任务
// @Description 重试失败的升级任务
// @Tags 升级管理
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/upgrades/{id}/retry [post]
func (c *UpgradeController) RetryUpgradeTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	taskID := uint(id)

	// 重试升级任务
	err = c.upgradeService.RetryUpgradeTask(taskID)
	if err != nil {
		render.Render(w, r, BadRequestResponse("重试升级任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("升级任务已重新启动", nil))
}

// DeleteUpgradeTask 删除升级任务
// @Summary 删除升级任务
// @Description 删除指定的升级任务记录
// @Tags 升级管理
// @Produce json
// @Param id path int true "任务ID"
// @Success 200 {object} APIResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/upgrades/{id} [delete]
func (c *UpgradeController) DeleteUpgradeTask(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		render.Render(w, r, BadRequestResponse("无效的任务ID", err))
		return
	}

	taskID := uint(id)

	// 删除升级任务
	err = c.upgradeService.DeleteUpgradeTask(taskID)
	if err != nil {
		render.Render(w, r, InternalErrorResponse("删除升级任务失败", err))
		return
	}

	render.Render(w, r, SuccessResponse("升级任务已删除", map[string]interface{}{
		"deleted_task_id": taskID,
	}))
}

// 辅助函数

// getCurrentUserID 从请求上下文获取当前用户ID
func (c *UpgradeController) getCurrentUserID(r *http.Request) uint {
	// TODO: 实现从JWT token或session中获取用户ID
	// 目前返回默认用户ID 1
	return 1
}

// convertUpgradeTaskToResponse 转换升级任务为响应格式
func convertUpgradeTaskToResponse(task *models.UpgradeTask) *UpgradeTaskResponse {
	response := &UpgradeTaskResponse{
		ID:              task.ID,
		BoxID:           task.BoxID,
		Name:            task.Name,
		VersionFrom:     task.VersionFrom,
		VersionTo:       task.VersionTo,
		Status:          task.Status,
		Progress:        task.Progress,
		ProgramFile:     task.ProgramFile,
		Force:           task.Force,
		ErrorMessage:    task.ErrorMessage,
		CanRollback:     task.CanRollback,
		RollbackVersion: task.RollbackVersion,
		CreatedAt:       task.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	// 设置盒子名称
	if task.Box.ID != 0 {
		response.BoxName = task.Box.Name
	}

	// 设置时间字段
	if task.StartedAt != nil {
		startedAt := task.StartedAt.Format("2006-01-02 15:04:05")
		response.StartedAt = &startedAt
	}

	if task.CompletedAt != nil {
		completedAt := task.CompletedAt.Format("2006-01-02 15:04:05")
		response.CompletedAt = &completedAt
	}

	return response
}

// convertBatchUpgradeTaskToResponse 转换批量升级任务为响应格式
func convertBatchUpgradeTaskToResponse(task *models.BatchUpgradeTask) *BatchUpgradeTaskResponse {
	response := &BatchUpgradeTaskResponse{
		ID:             task.ID,
		Name:           task.Name,
		BoxIDs:         []uint(task.BoxIDs),
		VersionTo:      task.VersionTo,
		Status:         task.Status,
		TotalBoxes:     task.TotalBoxes,
		CompletedBoxes: task.CompletedBoxes,
		FailedBoxes:    task.FailedBoxes,
		ProgramFile:    task.ProgramFile,
		Force:          task.Force,
		CreatedAt:      task.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	// 设置时间字段
	if task.StartedAt != nil {
		startedAt := task.StartedAt.Format("2006-01-02 15:04:05")
		response.StartedAt = &startedAt
	}

	if task.CompletedAt != nil {
		completedAt := task.CompletedAt.Format("2006-01-02 15:04:05")
		response.CompletedAt = &completedAt
	}

	return response
}
