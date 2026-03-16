/*
 * @module api/controllers/workflow_instance_controller
 * @description 工作流实例控制器实现
 * @architecture API层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow HTTP Request -> Controller -> Service -> Repository -> Database
 * @rules 实现工作流实例相关的RESTful API接口
 * @dependencies service, models
 * @refs 业务编排引擎需求文档.md 6.2节
 */

package controllers

import (
	"box-manage-service/service"
	"net/http"
	"strconv"
	"time"

	"encoding/json"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// WorkflowInstanceController 工作流实例控制器
type WorkflowInstanceController struct {
	instanceService service.WorkflowInstanceService
	executorService service.WorkflowExecutorService
}

// NewWorkflowInstanceController 创建工作流实例控制器实例
func NewWorkflowInstanceController(
	instanceService service.WorkflowInstanceService,
	executorService service.WorkflowExecutorService,
) *WorkflowInstanceController {
	return &WorkflowInstanceController{
		instanceService: instanceService,
		executorService: executorService,
	}
}

// CreateInstanceRequest 创建实例请求
type CreateInstanceRequest struct {
	WorkflowID     uint                   `json:"workflow_id" binding:"required"`
	BoxID          *uint                  `json:"box_id"`
	InputVariables map[string]interface{} `json:"input_variables"`
}

// CreateInstance 创建工作流实例
// @Summary 创建工作流实例
// @Description 从工作流定义创建新的执行实例，可指定输入变量和执行盒子
// @Tags 工作流api-工作流实例
// @Accept json
// @Produce json
// @Param request body CreateInstanceRequest true "创建实例请求，包含workflow_id、box_id（可选）、input_variables（可选）"
// @Success 200 {object} APIResponse{data=models.WorkflowInstance} "创建成功，返回实例对象"
// @Failure 400 {object} APIResponse "参数错误或工作流不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-instances [post]
func (c *WorkflowInstanceController) CreateInstance(w http.ResponseWriter, r *http.Request) {
	var req CreateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "参数错误", err))
		return
	}

	instance, err := c.instanceService.CreateFromWorkflow(
		r.Context(),
		req.WorkflowID,
		req.BoxID,
		req.InputVariables,
		"manual",
	)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "创建工作流实例失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("创建工作流实例成功", instance))
}

// GetInstance 获取工作流实例详情
// @Summary 获取工作流实例详情
// @Description 根据ID获取工作流实例详情，包含执行状态、进度、节点实例等信息
// @Tags 工作流api-工作流实例
// @Accept json
// @Produce json
// @Param id path int true "实例ID"
// @Success 200 {object} APIResponse{data=models.WorkflowInstance} "获取成功，返回实例详情"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "实例不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-instances/{id} [get]
func (c *WorkflowInstanceController) GetInstance(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	instance, err := c.instanceService.GetByID(r.Context(), uint(id))
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, CreateErrorResponse(http.StatusNotFound, "工作流实例不存在", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取工作流实例成功", instance))
}

// ListInstances 列出工作流实例
// @Summary 列出工作流实例
// @Description 分页列出工作流实例，支持按工作流ID、盒子ID、状态等条件过滤
// @Tags 工作流api-工作流实例
// @Accept json
// @Produce json
// @Param page query int false "页码，从1开始" default(1)
// @Param page_size query int false "每页数量，最大100" default(10)
// @Param workflow_id query int false "工作流ID，过滤指定工作流的实例"
// @Param box_id query int false "盒子ID，过滤指定盒子上的实例"
// @Param status query string false "实例状态：pending/running/paused/completed/failed/cancelled"
// @Success 200 {object} APIResponse{data=[]models.WorkflowInstance} "获取成功，返回实例列表"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-instances [get]
func (c *WorkflowInstanceController) ListInstances(w http.ResponseWriter, r *http.Request) {
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

	var instances interface{}
	var total int64
	var err error

	// 根据查询参数过滤
	if workflowIDStr := r.URL.Query().Get("workflow_id"); workflowIDStr != "" {
		workflowID, _ := strconv.ParseUint(workflowIDStr, 10, 32)
		list, listErr := c.instanceService.FindByWorkflowID(r.Context(), uint(workflowID))
		instances = list
		total = int64(len(list))
		err = listErr
	} else if boxIDStr := r.URL.Query().Get("box_id"); boxIDStr != "" {
		boxID, _ := strconv.ParseUint(boxIDStr, 10, 32)
		list, listErr := c.instanceService.FindByBoxID(r.Context(), uint(boxID))
		instances = list
		total = int64(len(list))
		err = listErr
	} else {
		instances, total, err = c.instanceService.List(r.Context(), page, pageSize)
	}

	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取工作流实例列表失败", err))
		return
	}

	render.JSON(w, r, PaginatedSuccessResponse("获取工作流实例列表成功", instances, total, page, pageSize))
}

// ExecuteInstance 执行工作流实例
// @Summary 执行工作流实例
// @Description 启动工作流实例的执行，异步执行，立即返回。执行过程可通过日志和状态查询接口监控
// @Tags 工作流api-工作流实例
// @Accept json
// @Produce json
// @Param id path int true "实例ID"
// @Success 200 {object} APIResponse "执行请求已接受，工作流开始执行"
// @Failure 400 {object} APIResponse "无效的ID或实例状态不允许执行"
// @Failure 404 {object} APIResponse "实例不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-instances/{id}/execute [post]
func (c *WorkflowInstanceController) ExecuteInstance(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	// 异步执行工作流
	go func() {
		c.executorService.Execute(r.Context(), uint(id))
	}()

	render.JSON(w, r, SuccessResponse("工作流实例开始执行", nil))
}

// StopInstance 停止工作流实例
// @Summary 停止工作流实例
// @Description 停止正在执行的工作流实例，将状态改为cancelled。已完成的节点不会回滚
// @Tags 工作流api-工作流实例
// @Accept json
// @Produce json
// @Param id path int true "实例ID"
// @Success 200 {object} APIResponse "停止成功"
// @Failure 400 {object} APIResponse "无效的ID或实例状态不允许停止"
// @Failure 404 {object} APIResponse "实例不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-instances/{id}/stop [post]
func (c *WorkflowInstanceController) StopInstance(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.executorService.Stop(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "停止工作流实例失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("停止工作流实例成功", nil))
}

// PauseInstance 暂停工作流实例
// @Summary 暂停工作流实例
// @Description 暂停正在执行的工作流实例，将状态改为paused。当前节点执行完成后暂停，可通过resume接口恢复
// @Tags 工作流api-工作流实例
// @Accept json
// @Produce json
// @Param id path int true "实例ID"
// @Success 200 {object} APIResponse "暂停成功"
// @Failure 400 {object} APIResponse "无效的ID或实例状态不允许暂停"
// @Failure 404 {object} APIResponse "实例不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-instances/{id}/pause [post]
func (c *WorkflowInstanceController) PauseInstance(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.executorService.Pause(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "暂停工作流实例失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("暂停工作流实例成功", nil))
}

// ResumeInstance 恢复工作流实例
// @Summary 恢复工作流实例
// @Description 恢复已暂停的工作流实例，从暂停点继续执行。异步执行，立即返回
// @Tags 工作流api-工作流实例
// @Accept json
// @Produce json
// @Param id path int true "实例ID"
// @Success 200 {object} APIResponse "恢复请求已接受，工作流继续执行"
// @Failure 400 {object} APIResponse "无效的ID或实例状态不允许恢复"
// @Failure 404 {object} APIResponse "实例不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-instances/{id}/resume [post]
func (c *WorkflowInstanceController) ResumeInstance(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	// 异步恢复执行
	go func() {
		c.executorService.Resume(r.Context(), uint(id))
	}()

	render.JSON(w, r, SuccessResponse("恢复工作流实例成功", nil))
}

// GetStatistics 获取统计信息
// @Summary 获取工作流实例统计信息
// @Description 获取工作流实例的统计信息，包括总数、各状态数量、成功率、平均执行时间等
// @Tags 工作流api-工作流实例
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=map[string]interface{}} "获取成功，返回统计数据"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-instances/statistics [get]
func (c *WorkflowInstanceController) GetStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := c.instanceService.GetStatistics(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取统计信息失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取统计信息成功", stats))
}

// GetExecutionReport 获取执行报告
// @Summary 获取执行报告
// @Description 获取指定时间范围内的工作流执行报告，包括执行次数、成功率、失败原因分析等
// @Tags 工作流api-工作流实例
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期，格式：YYYY-MM-DD" format(date)
// @Param end_date query string true "结束日期，格式：YYYY-MM-DD" format(date)
// @Success 200 {object} APIResponse{data=map[string]interface{}} "获取成功，返回执行报告"
// @Failure 400 {object} APIResponse "日期格式错误或日期范围无效"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-instances/report [get]
func (c *WorkflowInstanceController) GetExecutionReport(w http.ResponseWriter, r *http.Request) {
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的开始日期", err))
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的结束日期", err))
		return
	}

	report, err := c.instanceService.GetExecutionReport(r.Context(), startDate, endDate)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取执行报告失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取执行报告成功", report))
}
