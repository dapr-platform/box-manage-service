/*
 * @module api/controllers/workflow_schedule_controller
 * @description 工作流调度控制器实现
 * @architecture API层
 * @documentReference 业务编排引擎需求文档.md
 * @stateFlow HTTP Request -> Controller -> Service -> Repository -> Database
 * @rules 实现工作流调度相关的RESTful API接口
 * @dependencies service, models
 * @refs 业务编排引擎需求文档.md 6.4节
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

// WorkflowScheduleController 工作流调度控制器
type WorkflowScheduleController struct {
	schedulerService service.WorkflowSchedulerService
}

// NewWorkflowScheduleController 创建工作流调度控制器实例
func NewWorkflowScheduleController(schedulerService service.WorkflowSchedulerService) *WorkflowScheduleController {
	return &WorkflowScheduleController{
		schedulerService: schedulerService,
	}
}

// CreateSchedule 创建调度配置
// @Summary 创建调度配置
// @Description 创建新的工作流调度配置，支持cron表达式定时调度和手动触发
// @Tags 工作流调度
// @Accept json
// @Produce json
// @Param schedule body models.WorkflowSchedule true "调度配置信息，包含type（manual/cron）、cron_expression（cron类型必填）、input_variables等"
// @Success 200 {object} APIResponse{data=models.WorkflowSchedule} "创建成功，返回调度配置"
// @Failure 400 {object} APIResponse "参数错误或cron表达式无效"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-schedules [post]
func (c *WorkflowScheduleController) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var schedule models.WorkflowSchedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "参数错误", err))
		return
	}

	if err := c.schedulerService.CreateSchedule(r.Context(), &schedule); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "创建调度配置失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("创建调度配置成功", schedule))
}

// GetSchedule 获取调度配置详情
// @Summary 获取调度配置详情
// @Description 根据ID获取调度配置详情，包含调度规则、执行历史等信息
// @Tags 工作流调度
// @Accept json
// @Produce json
// @Param id path int true "调度配置ID"
// @Success 200 {object} APIResponse{data=models.WorkflowSchedule} "获取成功，返回调度配置"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "调度配置不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-schedules/{id} [get]
func (c *WorkflowScheduleController) GetSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	schedule, err := c.schedulerService.GetSchedule(r.Context(), uint(id))
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, CreateErrorResponse(http.StatusNotFound, "调度配置不存在", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取调度配置成功", schedule))
}

// UpdateSchedule 更新调度配置
// @Summary 更新调度配置
// @Description 更新调度配置信息，包括cron表达式、输入变量等。更新后会重新计算下次执行时间
// @Tags 工作流调度
// @Accept json
// @Produce json
// @Param id path int true "调度配置ID"
// @Param schedule body models.WorkflowSchedule true "调度配置信息"
// @Success 200 {object} APIResponse{data=models.WorkflowSchedule} "更新成功"
// @Failure 400 {object} APIResponse "参数错误或cron表达式无效"
// @Failure 404 {object} APIResponse "调度配置不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-schedules/{id} [put]
func (c *WorkflowScheduleController) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	var schedule models.WorkflowSchedule
	if err := json.NewDecoder(r.Body).Decode(&schedule); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "参数错误", err))
		return
	}

	schedule.ID = uint(id)
	if err := c.schedulerService.UpdateSchedule(r.Context(), &schedule); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "更新调度配置失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("更新调度配置成功", schedule))
}

// DeleteSchedule 删除调度配置
// @Summary 删除调度配置
// @Description 删除调度配置，删除后该调度将不再执行
// @Tags 工作流调度
// @Accept json
// @Produce json
// @Param id path int true "调度配置ID"
// @Success 200 {object} APIResponse "删除成功"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "调度配置不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-schedules/{id} [delete]
func (c *WorkflowScheduleController) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.schedulerService.DeleteSchedule(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "删除调度配置失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("删除调度配置成功", nil))
}

// ListSchedules 列出调度配置
// @Summary 列出调度配置
// @Description 列出指定工作流的所有调度配置
// @Tags 工作流调度
// @Accept json
// @Produce json
// @Param workflow_id query int true "工作流ID，必填"
// @Success 200 {object} APIResponse{data=[]models.WorkflowSchedule} "获取成功，返回调度配置列表"
// @Failure 400 {object} APIResponse "工作流ID无效"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-schedules [get]
func (c *WorkflowScheduleController) ListSchedules(w http.ResponseWriter, r *http.Request) {
	workflowIDStr := r.URL.Query().Get("workflow_id")
	workflowID, err := strconv.ParseUint(workflowIDStr, 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的工作流ID", err))
		return
	}

	schedules, err := c.schedulerService.ListSchedules(r.Context(), uint(workflowID))
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "获取调度配置列表失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("获取调度配置列表成功", schedules))
}

// EnableSchedule 启用调度配置
// @Summary 启用调度配置
// @Description 启用调度配置，启用后调度器会按照配置的规则自动执行工作流
// @Tags 工作流调度
// @Accept json
// @Produce json
// @Param id path int true "调度配置ID"
// @Success 200 {object} APIResponse "启用成功"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "调度配置不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-schedules/{id}/enable [post]
func (c *WorkflowScheduleController) EnableSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.schedulerService.EnableSchedule(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "启用调度配置失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("启用调度配置成功", nil))
}

// DisableSchedule 禁用调度配置
// @Summary 禁用调度配置
// @Description 禁用调度配置，禁用后调度器将不再自动执行该工作流
// @Tags 工作流调度
// @Accept json
// @Produce json
// @Param id path int true "调度配置ID"
// @Success 200 {object} APIResponse "禁用成功"
// @Failure 400 {object} APIResponse "无效的ID"
// @Failure 404 {object} APIResponse "调度配置不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-schedules/{id}/disable [post]
func (c *WorkflowScheduleController) DisableSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.schedulerService.DisableSchedule(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "禁用调度配置失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("禁用调度配置成功", nil))
}

// TriggerManual 手动触发
// @Summary 手动触发工作流
// @Description 手动触发工作流执行，不受调度配置的cron表达式限制，立即创建实例并执行
// @Tags 工作流调度
// @Accept json
// @Produce json
// @Param id path int true "调度配置ID"
// @Success 200 {object} APIResponse "触发成功，工作流开始执行"
// @Failure 400 {object} APIResponse "无效的ID或调度配置已禁用"
// @Failure 404 {object} APIResponse "调度配置不存在"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /api/v1/workflow-schedules/{id}/trigger [post]
func (c *WorkflowScheduleController) TriggerManual(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, CreateErrorResponse(http.StatusBadRequest, "无效的ID", err))
		return
	}

	if err := c.schedulerService.TriggerManual(r.Context(), uint(id)); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, CreateErrorResponse(http.StatusInternalServerError, "触发工作流失败", err))
		return
	}

	render.JSON(w, r, SuccessResponse("触发工作流成功", nil))
}
