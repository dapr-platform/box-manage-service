/*
 * @module api/controllers/workflow_controller
 * @description 工作流控制器实现
 * @architecture API层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow HTTP Request -> Controller -> Service -> Repository -> Database
 * @rules 实现工作流相关的RESTful API接口
 * @dependencies service, models
 * @refs 业务编排引擎需求文档.md 6.1节
 */

package controllers

import (
	"box-manage-service/models"
	"box-manage-service/service"
	"net/http"
	"strconv"

	"encoding/json"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// WorkflowController 工作流控制器
type WorkflowController struct {
	workflowService service.WorkflowService
}

// NewWorkflowController 创建工作流控制器实例
func NewWorkflowController(workflowService service.WorkflowService) *WorkflowController {
	return &WorkflowController{
		workflowService: workflowService,
	}
}

// CreateWorkflow 创建工作流
// @Summary 创建工作流
// @Description 创建新的工作流定义，包含节点、连接线、变量等完整结构
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param workflow body models.Workflow true "工作流信息，包含structure_json字段定义流程结构"
// @Success 200 {object} APIResponse{data=models.Workflow} "创建成功，返回工作流对象"
// @Failure 400 {object} ErrorResponse "参数错误或结构验证失败"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/workflows [post]
func (c *WorkflowController) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var workflow models.Workflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "参数错误", err))
		return
	}

	if err := c.workflowService.Create(r.Context(), &workflow); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "创建工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("创建工作流成功", workflow))
}

// GetWorkflow 获取工作流详情
// @Summary 获取工作流详情
// @Description 根据ID获取工作流详情，包含完整的流程结构、节点定义、连接线等信息
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param id path int true "工作流ID"
// @Success 200 {object} APIResponse{data=models.Workflow} "获取成功，返回工作流详情"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/{id} [get]
func (c *WorkflowController) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	workflow, err := c.workflowService.GetByID(r.Context(), uint(id))
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, CreateErrorResponse(http.StatusNotFound, "工作流不存在", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取工作流成功", workflow))
}

// UpdateWorkflow 更新工作流
// @Summary 更新工作流
// @Description 更新工作流信息，包括基本信息和流程结构。更新后会自动同步到关联的定义表
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param id path int true "工作流ID"
// @Param workflow body models.Workflow true "工作流信息，包含更新后的structure_json"
// @Success 200 {object} APIResponse{data=models.Workflow} "更新成功"
// @Failure 400 {object} APIResponse "参数错误或结构验证失败"
// @Failure 404 {object} APIResponse "工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/{id} [put]
func (c *WorkflowController) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	var workflow models.Workflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "参数错误", err))
		return
	}

	workflow.ID = uint(id)
	if err := c.workflowService.Update(r.Context(), &workflow); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "更新工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("更新工作流成功", workflow))
}

// DeleteWorkflow 删除工作流
// @Summary 删除工作流
// @Description 软删除工作流，不会真正删除数据，只是标记为已删除。删除前会检查是否有运行中的实例
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param id path int true "工作流ID"
// @Success 200 {object} APIResponse "删除成功"
// @Failure 400 {object} APIResponse "无效的ID或工作流正在使用中"
// @Failure 404 {object} APIResponse "工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/{id} [delete]
func (c *WorkflowController) DeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.workflowService.Delete(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "删除工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("删除工作流成功", nil))
}

// ListWorkflows 列出工作流
// @Summary 列出工作流
// @Description 分页列出工作流列表，支持按状态、分类等条件过滤
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param page query int false "页码，从1开始" default(1)
// @Param page_size query int false "每页数量，最大100" default(10)
// @Param status query string false "工作流状态：draft/published/archived"
// @Param category query string false "工作流分类"
// @Success 200 {object} APIResponse{data=[]models.Workflow} "获取成功，返回工作流列表和分页信息"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows [get]
func (c *WorkflowController) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, _ := strconv.Atoi(pageStr)
	pageSizeStr := r.URL.Query().Get("page_size")
	if pageSizeStr == "" {
		pageSizeStr = "10"
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)

	workflows, total, err := c.workflowService.List(r.Context(), page, pageSize)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取工作流列表失败", err))
		return
	}

	render.JSON(w, r, PaginatedSuccessResponse("获取工作流列表成功", workflows, total, page, pageSize))
}

// PublishWorkflow 发布工作流
// @Summary 发布工作流
// @Description 发布工作流，将状态从draft改为published。发布前会验证流程结构的完整性和正确性
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param id path int true "工作流ID"
// @Success 200 {object} APIResponse "发布成功"
// @Failure 400 {object} APIResponse "工作流结构验证失败或状态不允许发布"
// @Failure 404 {object} APIResponse "工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/{id}/publish [post]
func (c *WorkflowController) PublishWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.workflowService.Publish(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "发布工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("发布工作流成功", nil))
}

// ArchiveWorkflow 归档工作流
// @Summary 归档工作流
// @Description 归档工作流，将状态改为archived。归档后的工作流不能再执行，但可以查看历史记录
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param id path int true "工作流ID"
// @Success 200 {object} APIResponse "归档成功"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/{id}/archive [post]
func (c *WorkflowController) ArchiveWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.workflowService.Archive(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "归档工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("归档工作流成功", nil))
}

// EnableWorkflow 启用工作流
// @Summary 启用工作流
// @Description 启用工作流，设置is_enabled为true。启用后的工作流可以被调度和执行
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param id path int true "工作流ID"
// @Success 200 {object} APIResponse "启用成功"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/{id}/enable [post]
func (c *WorkflowController) EnableWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.workflowService.Enable(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "启用工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("启用工作流成功", nil))
}

// DisableWorkflow 禁用工作流
// @Summary 禁用工作流
// @Description 禁用工作流，设置is_enabled为false。禁用后的工作流不能被调度和执行
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param id path int true "工作流ID"
// @Success 200 {object} APIResponse "禁用成功"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/{id}/disable [post]
func (c *WorkflowController) DisableWorkflow(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.workflowService.Disable(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "禁用工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("禁用工作流成功", nil))
}

// CreateNewVersion 创建新版本
// @Summary 创建工作流新版本
// @Description 基于现有工作流创建新版本，版本号自动递增。新版本默认为draft状态
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param key_name path string true "工作流标识（key_name）"
// @Param workflow body models.Workflow true "新版本的工作流信息"
// @Success 200 {object} APIResponse{data=models.Workflow} "创建成功，返回新版本工作流"
// @Failure 400 {object} APIResponse "参数错误"
// @Failure 404 {object} APIResponse "原工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/{key_name}/versions [post]
func (c *WorkflowController) CreateNewVersion(w http.ResponseWriter, r *http.Request) {
	keyName := chi.URLParam(r, "key_name")

	var workflow models.Workflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "参数错误", err))
		return
	}

	if err := c.workflowService.CreateNewVersion(r.Context(), keyName, &workflow); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "创建新版本失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("创建新版本成功", workflow))
}

// GetAllVersions 获取所有版本
// @Summary 获取工作流所有版本
// @Description 获取指定工作流的所有版本列表，按版本号降序排列
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param id path int true "工作流ID"
// @Success 200 {object} APIResponse{data=[]models.Workflow} "获取成功，返回版本列表"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/{id}/versions [get]
func (c *WorkflowController) GetAllVersions(w http.ResponseWriter, r *http.Request) {
	keyName := chi.URLParam(r, "key_name")

	workflows, err := c.workflowService.GetAllVersions(r.Context(), keyName)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取版本列表失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取版本列表成功", workflows))
}

// SearchWorkflows 搜索工作流
// @Summary 搜索工作流
// @Description 根据关键词搜索工作流，支持按名称、描述、标签等字段模糊匹配
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Param keyword query string true "搜索关键词，支持模糊匹配"
// @Param page query int false "页码，从1开始" default(1)
// @Param page_size query int false "每页数量，最大100" default(10)
// @Success 200 {object} APIResponse{data=[]models.Workflow} "搜索成功，返回匹配的工作流列表"
// @Failure 400 {object} APIResponse "参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/search [get]
func (c *WorkflowController) SearchWorkflows(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, _ := strconv.Atoi(pageStr)
	pageSizeStr := r.URL.Query().Get("page_size")
	if pageSizeStr == "" {
		pageSizeStr = "10"
	}
	pageSize, _ := strconv.Atoi(pageSizeStr)

	workflows, total, err := c.workflowService.Search(r.Context(), keyword, page, pageSize)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "搜索工作流失败", err))
		return
	}

	render.JSON(w, r, PaginatedSuccessResponse("搜索工作流成功", workflows, total, page, pageSize))
}

// GetStatistics 获取统计信息
// @Summary 获取工作流统计信息
// @Description 获取工作流的统计信息，包括总数、各状态数量、分类分布等
// @Tags 工作流管理
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=map[string]interface{}} "获取成功，返回统计数据"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflows/statistics [get]
func (c *WorkflowController) GetStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := c.workflowService.GetStatistics(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取统计信息失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取统计信息成功", stats))
}
